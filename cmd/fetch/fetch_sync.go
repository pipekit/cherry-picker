package fetch

import (
	"log/slog"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/github"
)

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
