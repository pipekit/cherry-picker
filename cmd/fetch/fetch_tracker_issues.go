package fetch

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/github"
)

// updateTrackerIssues detects and stores tracker issues for all branches
func updateTrackerIssues(ctx context.Context, config *cmd.Config, client *github.Client) bool {
	// Initialize TrackerIssues map if needed
	if config.TrackerIssues == nil {
		config.TrackerIssues = make(map[string]int)
	}

	// Get all unique branches from tracked PRs
	branches := getAllBranches(config)
	if len(branches) == 0 {
		return false
	}

	updated := false
	// Only detect for branches that don't already have a tracker issue
	for _, branch := range branches {
		if _, exists := config.TrackerIssues[branch]; !exists {
			if detectTrackerIssueForBranch(ctx, client, config, branch) {
				updated = true
			}
		}
	}

	return updated
}

// getAllBranches returns all unique branch names from tracked PRs
func getAllBranches(config *cmd.Config) []string {
	branchSet := make(map[string]bool)
	for _, pr := range config.TrackedPRs {
		for branch := range pr.Branches {
			branchSet[branch] = true
		}
	}

	var branches []string
	for branch := range branchSet {
		branches = append(branches, branch)
	}
	return branches
}

// detectTrackerIssueForBranch detects the tracker issue for a specific branch
func detectTrackerIssueForBranch(ctx context.Context, client *github.Client, config *cmd.Config, branch string) bool {
	// Extract version from branch name
	// Expected format: "release-3.6" -> search for "Release v3.6 patch"
	version, ok := strings.CutPrefix(branch, "release-")
	if !ok {
		slog.Debug("Branch doesn't match expected format", "branch", branch)
		return false
	}

	searchText := "Release v" + version + " patch"
	slog.Debug("Searching for tracker issue", "branch", branch, "search_text", searchText)

	issues, err := client.SearchIssuesByText(ctx, searchText)
	if err != nil {
		slog.Debug("Failed to search for tracker issue", "branch", branch, "error", err)
		return false
	}

	if len(issues) == 0 {
		slog.Debug("No tracker issue found", "branch", branch)
		return false
	}

	// Use the first (most recently updated) issue
	trackerIssue := issues[0]
	config.TrackerIssues[branch] = trackerIssue.Number

	slog.Info("Found tracker issue", "branch", branch, "issue", trackerIssue.Number, "title", trackerIssue.Title)
	fmt.Printf("  Found tracker issue for %s: #%d\n", branch, trackerIssue.Number)

	return true
}
