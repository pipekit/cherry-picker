package fetch

import (
	"fmt"
	"os"
	"time"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/alan/cherry-picker/internal/github"
	"github.com/spf13/cobra"
)

// NewFetchCmd creates and returns the fetch command
func NewFetchCmd(globalConfigFile *string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) *cobra.Command {
	var (
		sinceDate string
	)

	command := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch new merged PRs from GitHub that need cherry-picking decisions",
		Long: `Fetch new merged PRs from the source branch since the last fetch date
(or a specified date) and interactively ask whether to pick or ignore each one.

Requires GITHUB_TOKEN environment variable to be set.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFetch(*globalConfigFile, sinceDate, loadConfig, saveConfig)
		},
	}

	command.Flags().StringVarP(&sinceDate, "since", "s", "", "Fetch PRs since this date (YYYY-MM-DD), defaults to last fetch date")

	return command
}

func runFetch(configFile, sinceDate string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) error {
	config, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	token, err := getGitHubToken()
	if err != nil {
		return err
	}

	since, err := determineSinceDate(sinceDate, config.LastFetchDate)
	if err != nil {
		return err
	}

	return fetchAndProcessPRs(configFile, config, since, token, saveConfig)
}

// getGitHubToken retrieves and validates the GitHub token
func getGitHubToken() (string, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return "", fmt.Errorf("GITHUB_TOKEN environment variable is required")
	}
	return token, nil
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
func fetchAndProcessPRs(configFile string, config *cmd.Config, since time.Time, token string, saveConfig func(string, *cmd.Config) error) error {
	// Update existing picked PRs with latest details
	if err := updateExistingPickedPRs(config); err != nil {
		fmt.Printf("Warning: Failed to update existing picked PRs: %v\n", err)
	}

	prs, err := fetchPRsFromGitHub(config, since)
	if err != nil {
		return err
	}

	if len(prs) == 0 {
		fmt.Println("No new merged PRs found.")
		return updateLastFetchDate(configFile, config, saveConfig)
	}

	newPRs := filterNewPRs(prs, config.TrackedPRs)
	if len(newPRs) == 0 {
		fmt.Println("No new PRs to review (all already tracked).")
		return updateLastFetchDate(configFile, config, saveConfig)
	}

	return processNewPRsInteractively(configFile, newPRs, config, saveConfig)
}

// fetchPRsFromGitHub fetches PRs from GitHub API
func fetchPRsFromGitHub(config *cmd.Config, since time.Time) ([]github.PR, error) {
	fmt.Printf("Fetching merged PRs from %s/%s since %s...\n", config.Org, config.Repo, since.Format("2006-01-02"))

	client, _, err := commands.InitializeGitHubClient()
	if err != nil {
		return nil, err
	}

	prs, err := client.GetMergedPRs(config.Org, config.Repo, config.SourceBranch, since)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PRs: %w", err)
	}

	return prs, nil
}

// filterNewPRs filters out PRs that are already tracked
func filterNewPRs(prs []github.PR, trackedPRs []cmd.TrackedPR) []github.PR {
	existingPRs := make(map[int]bool)
	for _, trackedPR := range trackedPRs {
		existingPRs[trackedPR.Number] = true
	}

	var newPRs []github.PR
	for _, pr := range prs {
		if !existingPRs[pr.Number] {
			newPRs = append(newPRs, pr)
		}
	}

	return newPRs
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

// processNewPRsInteractively handles the processing of new PRs based on their labels
func processNewPRsInteractively(configFile string, newPRs []github.PR, config *cmd.Config, saveConfig func(string, *cmd.Config) error) error {
	fmt.Printf("Found %d new merged PR(s) with cherry-pick labels:\n\n", len(newPRs))

	for _, pr := range newPRs {
		processSinglePR(pr, config)
	}

	return finalizePRProcessing(configFile, config, saveConfig)
}

// processSinglePR handles the processing of a single PR based on its cherry-pick labels
func processSinglePR(pr github.PR, config *cmd.Config) {
	fmt.Printf("PR #%d: %s\n", pr.Number, pr.Title)
	fmt.Printf("URL: %s\n", pr.URL)
	fmt.Printf("Cherry-pick labels: %v\n", pr.CherryPickFor)

	addPRForCherryPicking(config, pr)
	fmt.Printf("✓ Added PR #%d for cherry-picking to %v\n\n", pr.Number, pr.CherryPickFor)
}

// finalizePRProcessing saves the config and displays completion message
func finalizePRProcessing(configFile string, config *cmd.Config, saveConfig func(string, *cmd.Config) error) error {
	if err := updateLastFetchDate(configFile, config, saveConfig); err != nil {
		return err
	}

	fmt.Printf("Updated %s with new PRs and fetch date.\n", configFile)
	return nil
}

// addPRForCherryPicking adds a PR to be cherry-picked to branches specified by labels
func addPRForCherryPicking(config *cmd.Config, pr github.PR) {
	branches := make(map[string]cmd.BranchStatus)
	for _, branch := range pr.CherryPickFor {
		branches[branch] = cmd.BranchStatus{Status: "pending"}
	}

	config.TrackedPRs = append(config.TrackedPRs, cmd.TrackedPR{
		Number:   pr.Number,
		Title:    pr.Title,
		Ignored:  false,
		Branches: branches,
	})
}

// updateExistingPickedPRs updates details for existing picked PRs that are not merged
func updateExistingPickedPRs(config *cmd.Config) error {
	client, _, err := commands.InitializeGitHubClient()
	if err != nil {
		return err
	}

	for i := range config.TrackedPRs {
		trackedPR := &config.TrackedPRs[i]
		if trackedPR.Ignored {
			continue
		}

		// Check each branch for picked PRs
		for branch, status := range trackedPR.Branches {
			if status.Status == cmd.BranchStatusPicked && status.PR != nil {
				// Fetch latest details for picked PRs
				prDetails, err := client.GetPRWithDetails(config.Org, config.Repo, status.PR.Number)
				if err != nil {
					fmt.Printf("Warning: Failed to fetch details for pick PR #%d: %v\n", status.PR.Number, err)
					continue
				}

				// Update the PR details
				status.PR.CIStatus = cmd.ParseCIStatus(prDetails.CIStatus)
				status.PR.Title = prDetails.Title

				// Update status to merged if the PR was merged
				if prDetails.Merged {
					status.Status = cmd.BranchStatusMerged
				}

				trackedPR.Branches[branch] = status
			}
		}
	}

	return nil
}
