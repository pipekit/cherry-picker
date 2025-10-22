package fetch

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/alan/cherry-picker/internal/github"
)

// fetchAndProcessPRs handles the main logic of fetching and processing PRs
func fetchAndProcessPRs(ctx context.Context, configFile string, config *cmd.Config, since time.Time, saveConfig func(string, *cmd.Config) error) error {
	client, _, err := commands.InitializeGitHubClient(ctx, config)
	if err != nil {
		return err
	}

	allPRs, err := fetchPRsFromGitHub(ctx, config, since)
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
		if updateAllTrackedPRs(ctx, config, client) {
			configUpdated = true
		}

		// Check releases and mark cherry-picks as released
		slog.Info("Checking releases for merged cherry-picks")
		if updateReleasedStatus(ctx, config, client) {
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
func fetchPRsFromGitHub(ctx context.Context, config *cmd.Config, since time.Time) ([]github.PR, error) {
	slog.Info("Fetching merged PRs with cherry-pick labels", "org", config.Org, "repo", config.Repo)

	client, _, err := commands.InitializeGitHubClient(ctx, config)
	if err != nil {
		return nil, err
	}

	prs, err := client.GetMergedPRs(ctx, config.SourceBranch, since)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PRs: %w", err)
	}

	return prs, nil
}

// updateAllTrackedPRs updates all existing tracked PRs by checking their cherry-pick status
func updateAllTrackedPRs(ctx context.Context, config *cmd.Config, client *github.Client) bool {
	updated := false

	for i := range config.TrackedPRs {
		trackedPR := &config.TrackedPRs[i]
		slog.Info("Checking tracked PR", "pr", trackedPR.Number)

		cherryPickPRs, err := client.GetCherryPickPRsFromComments(ctx, trackedPR.Number)
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
		manualCherryPicks, err := client.SearchManualCherryPickPRs(ctx, trackedPR.Number, branches)
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
			if currentStatus.Status == cmd.BranchStatusMerged || currentStatus.Status == cmd.BranchStatusReleased {
				slog.Debug("Skipping finalized tracked PR", "pr", trackedPR.Number, "branch", branch, "status", currentStatus.Status)
				continue
			}
			slog.Info("Checking tracked PR", "pr", trackedPR.Number, "branch", branch)

			if cherryPick, cpExists := existingByBranch[branch]; cpExists {
				newStatus := determineBranchStatus(ctx, cherryPick, config, client, trackedPR)
				if currentStatus.Status != newStatus.Status ||
					(newStatus.PR != nil && (currentStatus.PR == nil || currentStatus.PR.Number != newStatus.PR.Number)) {
					trackedPR.Branches[branch] = newStatus
					updated = true
					slog.Info("Updated branch status", "pr", trackedPR.Number, "branch", branch,
						"old_status", currentStatus.Status, "new_status", newStatus.Status)
				} else if currentStatus.Status == cmd.BranchStatusPicked && currentStatus.PR != nil {
					prDetails, err := client.GetPRWithDetails(ctx, currentStatus.PR.Number)
					if err == nil {
						ciChanged := false
						if currentStatus.PR.CIStatus != cmd.ParseCIStatus(prDetails.CIStatus) {
							currentStatus.PR.CIStatus = cmd.ParseCIStatus(prDetails.CIStatus)
							ciChanged = true
						}
						if ciChanged {
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

// determineBranchStatus determines the status for a branch based on cherry-pick PR info
func determineBranchStatus(ctx context.Context, cherryPick github.CherryPickPR, _ *cmd.Config, client *github.Client, trackedPR *cmd.TrackedPR) cmd.BranchStatus {
	if cherryPick.Failed {
		return cmd.BranchStatus{Status: cmd.BranchStatusFailed}
	}

	prDetails, err := client.GetPRWithDetails(ctx, cherryPick.Number)
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
