package fetch

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/alan/cherry-picker/internal/github"
	"github.com/spf13/cobra"
)

// FetchCommand encapsulates the fetch command with common functionality
type FetchCommand struct {
	commands.BaseCommand
	SinceDate string
}

// NewFetchCmd creates and returns the fetch command
func NewFetchCmd(globalConfigFile *string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) *cobra.Command {
	fetchCmd := &FetchCommand{}

	command := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch new merged PRs from GitHub that need cherry-picking decisions",
		Long: `Fetch new merged PRs from the source branch since the last fetch date
(or a specified date) and interactively ask whether to pick or ignore each one.

Requires GITHUB_TOKEN environment variable to be set.`,
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			// Initialize base command
			fetchCmd.ConfigFile = globalConfigFile
			fetchCmd.LoadConfig = loadConfig
			fetchCmd.SaveConfig = saveConfig
			if err := fetchCmd.Init(); err != nil {
				return err
			}

			return fetchCmd.Run()
		},
	}

	command.Flags().StringVarP(&fetchCmd.SinceDate, "since", "s", "", "Fetch PRs since this date (YYYY-MM-DD), defaults to last fetch date")

	return command
}

// Run executes the fetch command
func (fc *FetchCommand) Run() error {
	since, err := determineSinceDate(fc.SinceDate, fc.Config.LastFetchDate)
	if err != nil {
		return err
	}

	return fetchAndProcessPRs(*fc.ConfigFile, fc.Config, since, fc.SaveConfig)
}

// determineSinceDate determines the date to fetch PRs from
func determineSinceDate(sinceDate string, lastFetchDate *time.Time) (time.Time, error) {
	if sinceDate != "" {
		since, err := time.Parse("2006-01-02", sinceDate)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid date format, use YYYY-MM-DD: %w", err)
		}
		return since, nil
	}

	if lastFetchDate != nil {
		return *lastFetchDate, nil
	}

	return time.Now().AddDate(0, 0, -30), nil
}

// fetchAndProcessPRs handles the main logic of fetching and processing PRs
func fetchAndProcessPRs(configFile string, config *cmd.Config, since time.Time, saveConfig func(string, *cmd.Config) error) error {
	client, _, err := commands.InitializeGitHubClient(config)
	if err != nil {
		return err
	}

	allPRs, err := fetchPRsFromGitHub(config, since)
	if err != nil {
		return err
	}

	slog.Info("Fetched PRs from GitHub", "count", len(allPRs))

	prByNumber := make(map[int]github.PR)
	for _, pr := range allPRs {
		prByNumber[pr.Number] = pr
	}

	configUpdated := false
	newPRsAdded := 0

	// Add new PRs from search results
	for _, pr := range allPRs {
		if !isPRTracked(config, pr.Number) {
			slog.Info("Found new PR", "number", pr.Number, "title", pr.Title, "url", pr.URL, "cherry_pick_labels", pr.CherryPickFor)
			addNewPR(config, pr)
			newPRsAdded++
		}
	}

	// Sync all tracked PRs with GitHub (including those without labels anymore)
	for i := range config.TrackedPRs {
		trackedPR := &config.TrackedPRs[i]
		// If PR is in search results, use that data
		if pr, found := prByNumber[trackedPR.Number]; found {
			if syncBranchesWithGitHub(config, pr) {
				configUpdated = true
			}
		} else {
			// PR not in search results - must have no cherry-pick labels
			// Create empty PR struct to sync (will remove all pending/failed branches)
			emptyPR := github.PR{
				Number:        trackedPR.Number,
				CherryPickFor: []string{}, // No labels
			}
			if syncBranchesWithGitHub(config, emptyPR) {
				configUpdated = true
			}
		}
	}

	// Remove PRs with no branches left
	prsRemoved := removeEmptyPRs(config)
	if prsRemoved > 0 {
		slog.Info("Removed PRs with no branches", "count", prsRemoved)
		configUpdated = true
	}

	if newPRsAdded > 0 {
		slog.Info("Added new PRs", "count", newPRsAdded)
	}
	if len(config.TrackedPRs) > 0 {
		slog.Info("Updating tracked PRs", "count", len(config.TrackedPRs))
		if updateAllTrackedPRs(config, client) {
			configUpdated = true
		}
	}

	if configUpdated || newPRsAdded > 0 {
		slog.Info("Configuration updated", "total_tracked_prs", len(config.TrackedPRs))
	} else {
		slog.Info("No changes detected")
	}

	return updateLastFetchDate(configFile, config, saveConfig)
}

// fetchPRsFromGitHub fetches PRs from GitHub API
func fetchPRsFromGitHub(config *cmd.Config, since time.Time) ([]github.PR, error) {
	slog.Info("Fetching merged PRs with cherry-pick labels", "org", config.Org, "repo", config.Repo)

	client, _, err := commands.InitializeGitHubClient(config)
	if err != nil {
		return nil, err
	}

	prs, err := client.GetMergedPRs(config.SourceBranch, since)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PRs: %w", err)
	}

	return prs, nil
}

// updateAllTrackedPRs updates all existing tracked PRs by checking their cherry-pick status
func updateAllTrackedPRs(config *cmd.Config, client *github.Client) bool {
	updated := false

	for i := range config.TrackedPRs {
		trackedPR := &config.TrackedPRs[i]
		slog.Info("Checking tracked PR", "pr", trackedPR.Number)

		cherryPickPRs, err := client.GetCherryPickPRsFromComments(trackedPR.Number)
		if err != nil {
			slog.Warn("Failed to fetch cherry-pick PRs from comments", "pr", trackedPR.Number, "error", err)
			cherryPickPRs = []github.CherryPickPR{}
		}

		// Get list of branches we're tracking for this PR
		var branches []string
		for branch := range trackedPR.Branches {
			branches = append(branches, branch)
		}

		// Search for manual cherry-pick PRs by title
		manualCherryPicks, err := client.SearchManualCherryPickPRs(trackedPR.Number, branches)
		if err != nil {
			slog.Warn("Failed to search for manual cherry-pick PRs", "pr", trackedPR.Number, "error", err)
		} else {
			// Merge manual cherry-picks with bot cherry-picks
			cherryPickPRs = append(cherryPickPRs, manualCherryPicks...)
		}

		existingByBranch := make(map[string]github.CherryPickPR)
		for _, cp := range cherryPickPRs {
			existingByBranch[cp.Branch] = cp
		}

		for branch, currentStatus := range trackedPR.Branches {
			if currentStatus.Status == cmd.BranchStatusMerged {
				slog.Info("Skipping merged tracked PR", "pr", trackedPR.Number, "branch", branch)
				continue
			}
			slog.Info("Checking tracked PR", "pr", trackedPR.Number, "branch", branch)

			if cherryPick, cpExists := existingByBranch[branch]; cpExists {
				newStatus := determineBranchStatus(cherryPick, config, client, trackedPR)
				if currentStatus.Status != newStatus.Status ||
					(newStatus.PR != nil && (currentStatus.PR == nil || currentStatus.PR.Number != newStatus.PR.Number)) {
					trackedPR.Branches[branch] = newStatus
					updated = true
					slog.Info("Updated branch status", "pr", trackedPR.Number, "branch", branch,
						"old_status", currentStatus.Status, "new_status", newStatus.Status)
				} else if currentStatus.Status == cmd.BranchStatusPicked && currentStatus.PR != nil {
					prDetails, err := client.GetPRWithDetails(currentStatus.PR.Number)
					if err == nil {
						ciChanged := false
						if currentStatus.PR.CIStatus != cmd.ParseCIStatus(prDetails.CIStatus) {
							currentStatus.PR.CIStatus = cmd.ParseCIStatus(prDetails.CIStatus)
							ciChanged = true
						}
						if prDetails.Merged && currentStatus.Status != cmd.BranchStatusMerged {
							currentStatus.Status = cmd.BranchStatusMerged
							trackedPR.Branches[branch] = currentStatus
							updated = true
							slog.Info("Cherry-pick PR merged", "pr", trackedPR.Number, "branch", branch, "cherry_pick_pr", currentStatus.PR.Number)
						} else if ciChanged {
							trackedPR.Branches[branch] = currentStatus
							updated = true
							slog.Info("Cherry-pick PR CI status updated", "pr", trackedPR.Number, "branch", branch, "ci_status", currentStatus.PR.CIStatus)
						}
					}
				}
			} else {
				slog.Info("No existing Cherry-pick for tracked PR", "pr", trackedPR.Number, "branch", branch)
			}
		}
	}

	return updated
}

// isPRTracked checks if a PR is already being tracked
func isPRTracked(config *cmd.Config, prNumber int) bool {
	for _, trackedPR := range config.TrackedPRs {
		if trackedPR.Number == prNumber {
			return true
		}
	}
	return false
}

// removeEmptyPRs removes PRs that have no branches left
// Returns the number of PRs removed
func removeEmptyPRs(config *cmd.Config) int {
	var remaining []cmd.TrackedPR
	removed := 0

	for _, pr := range config.TrackedPRs {
		if len(pr.Branches) == 0 {
			slog.Info("Removing PR with no branches", "pr", pr.Number)
			removed++
		} else {
			remaining = append(remaining, pr)
		}
	}

	config.TrackedPRs = remaining
	return removed
}

// syncBranchesWithGitHub syncs tracked branches with current GitHub labels
// Returns true if any changes were made
func syncBranchesWithGitHub(config *cmd.Config, pr github.PR) bool {
	updated := false

	for i := range config.TrackedPRs {
		if config.TrackedPRs[i].Number != pr.Number {
			continue
		}

		trackedPR := &config.TrackedPRs[i]

		// Build set of branches from GitHub labels
		githubBranches := make(map[string]bool)
		for _, branch := range pr.CherryPickFor {
			githubBranches[branch] = true
		}

		// Add new branches from GitHub labels
		for branch := range githubBranches {
			if _, exists := trackedPR.Branches[branch]; !exists {
				slog.Info("Adding new branch from label", "pr", pr.Number, "branch", branch)
				if trackedPR.Branches == nil {
					trackedPR.Branches = make(map[string]cmd.BranchStatus)
				}
				trackedPR.Branches[branch] = cmd.BranchStatus{Status: cmd.BranchStatusPending}
				updated = true
			}
		}

		// Remove branches that no longer have labels on GitHub (unless already picked/merged)
		for branch, status := range trackedPR.Branches {
			if !githubBranches[branch] {
				// Only remove if still pending or failed - keep picked/merged for history
				if status.Status == cmd.BranchStatusPending || status.Status == cmd.BranchStatusFailed {
					slog.Info("Removing branch - label removed from GitHub", "pr", pr.Number, "branch", branch)
					delete(trackedPR.Branches, branch)
					updated = true
				}
			}
		}

		break
	}

	return updated
}

// addNewPR adds a new PR to the config without checking cherry-pick status
func addNewPR(config *cmd.Config, pr github.PR) {
	branches := make(map[string]cmd.BranchStatus)
	for _, branch := range pr.CherryPickFor {
		branches[branch] = cmd.BranchStatus{Status: cmd.BranchStatusPending}
	}

	config.TrackedPRs = append(config.TrackedPRs, cmd.TrackedPR{
		Number:   pr.Number,
		Title:    pr.Title,
		Branches: branches,
	})
}

// determineBranchStatus determines the status for a branch based on cherry-pick PR info
func determineBranchStatus(cherryPick github.CherryPickPR, config *cmd.Config, client *github.Client, trackedPR *cmd.TrackedPR) cmd.BranchStatus {
	if cherryPick.Failed {
		return cmd.BranchStatus{Status: cmd.BranchStatusFailed}
	}

	prDetails, err := client.GetPRWithDetails(cherryPick.Number)
	if err != nil {
		slog.Warn("Failed to fetch PR details", "pr", cherryPick.Number, "error", err)
		return cmd.BranchStatus{
			Status: cmd.BranchStatusPicked,
			PR: &cmd.PickPR{
				Number:   cherryPick.Number,
				Title:    fmt.Sprintf("%s (cherry-pick %s)", trackedPR.Title, cherryPick.Branch),
				CIStatus: "unknown",
			},
		}
	}

	status := cmd.BranchStatusPicked
	if prDetails.Merged {
		status = cmd.BranchStatusMerged
	}

	return cmd.BranchStatus{
		Status: status,
		PR: &cmd.PickPR{
			Number:   prDetails.Number,
			Title:    prDetails.Title,
			CIStatus: cmd.ParseCIStatus(prDetails.CIStatus),
		},
	}
}

// updateLastFetchDate updates the last fetch date and saves the config
func updateLastFetchDate(configFile string, config *cmd.Config, saveConfig func(string, *cmd.Config) error) error {
	now := time.Now()
	config.LastFetchDate = &now
	if err := saveConfig(configFile, config); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}
	return nil
}
