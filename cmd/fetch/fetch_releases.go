package fetch

import (
	"context"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/github"
)

// cherryPickPattern matches the line added by 'git cherry-pick -x'
// Example: "(cherry picked from commit abc123def456)"
var cherryPickPattern = regexp.MustCompile(`\(cherry picked from commit ([a-f0-9]+)\)`)

// botCherryPickPattern matches GitHub bot cherry-pick PR format
// Example: "(cherry-pick #15033 for 3.6)"
var botCherryPickPattern = regexp.MustCompile(`\(cherry-pick #(\d+) for [\d.]+\)`)

// prNumberPattern matches PR numbers in commit messages
// Example: "Fix bug (#123)" or "Merge pull request #456"
var prNumberPattern = regexp.MustCompile(`#(\d+)`)

// updateReleasedStatus checks all releases and marks cherry-pick PRs as released
func updateReleasedStatus(ctx context.Context, config *cmd.Config, client *github.Client) bool {
	updated := false

	// Get all releases
	allReleases, err := client.ListReleases(ctx)
	if err != nil {
		slog.Warn("Failed to fetch releases", "error", err)
		return false
	}

	if len(allReleases) == 0 {
		slog.Info("No releases found")
		return false
	}

	slog.Info("Fetched all releases", "total_count", len(allReleases))

	// Initialize the last checked release map if needed
	if config.LastCheckedRelease == nil {
		config.LastCheckedRelease = make(map[string]string)
	}

	// First, collect all unique branches with merged PRs
	// and compute unchecked releases once per branch
	type branchReleases struct {
		relevantReleases  []github.Release
		uncheckedReleases []github.Release
		lastChecked       string
	}
	branchReleasesMap := make(map[string]*branchReleases)

	// Collect all unique branches that have merged PRs
	for i := range config.TrackedPRs {
		trackedPR := &config.TrackedPRs[i]
		for branchName, branchStatus := range trackedPR.Branches {
			// Only check merged PRs that haven't been marked as released yet
			if branchStatus.Status != cmd.BranchStatusMerged {
				continue
			}
			if branchStatus.PR == nil {
				continue
			}

			// Only compute releases for this branch once
			if _, exists := branchReleasesMap[branchName]; !exists {
				// Filter releases to only those relevant for this branch
				relevantReleases := filterReleasesForBranch(allReleases, branchName)
				lastChecked := config.LastCheckedRelease[branchName]
				uncheckedReleases := filterUncheckedReleases(relevantReleases, lastChecked)

				branchReleasesMap[branchName] = &branchReleases{
					relevantReleases:  relevantReleases,
					uncheckedReleases: uncheckedReleases,
					lastChecked:       lastChecked,
				}
			}
		}
	}

	// Log summary of branches to check (once per branch instead of per PR)
	for branchName, br := range branchReleasesMap {
		if len(br.relevantReleases) == 0 {
			slog.Debug("No relevant releases for branch", "branch", branchName)
		} else if len(br.uncheckedReleases) == 0 {
			slog.Debug("No new releases to check for branch", "branch", branchName, "last_checked", br.lastChecked)
		} else {
			slog.Debug("Checking new releases", "branch", branchName, "new_count", len(br.uncheckedReleases), "total_relevant", len(br.relevantReleases))
		}
	}

	// Track which branches we've checked (to update last checked release)
	branchesChecked := make(map[string]string) // branch -> latest release checked

	// Now check each PR against the releases for its branches
	for i := range config.TrackedPRs {
		trackedPR := &config.TrackedPRs[i]

		for branchName, branchStatus := range trackedPR.Branches {
			// Only check merged PRs that haven't been marked as released yet
			if branchStatus.Status != cmd.BranchStatusMerged {
				continue
			}
			if branchStatus.PR == nil {
				continue
			}

			// Get the pre-computed releases for this branch
			br, exists := branchReleasesMap[branchName]
			if !exists || len(br.uncheckedReleases) == 0 {
				continue
			}

			// Track the latest release we're checking for this branch
			if len(br.uncheckedReleases) > 0 {
				// Releases are sorted newest first, so first one is the latest
				branchesChecked[branchName] = br.uncheckedReleases[0].TagName
			}

			// Check if this cherry-pick PR's commit is in any release
			if isInRelease(ctx, client, br.uncheckedReleases, br.lastChecked, trackedPR.Number) {
				slog.Info("Cherry-pick found in release", "pr", trackedPR.Number, "branch", branchName, "cherry_pick_pr", branchStatus.PR.Number)
				branchStatus.Status = cmd.BranchStatusReleased
				trackedPR.Branches[branchName] = branchStatus
				updated = true
			}
		}
	}

	// Update last checked releases for all branches we checked
	for branch, latestRelease := range branchesChecked {
		config.LastCheckedRelease[branch] = latestRelease
		updated = true // Config changed
		slog.Debug("Updated last checked release", "branch", branch, "release", latestRelease)
	}

	return updated
}

// filterUncheckedReleases returns only releases newer than the last checked release
// Assumes releases are sorted newest first
func filterUncheckedReleases(releases []github.Release, lastChecked string) []github.Release {
	if lastChecked == "" {
		// Never checked before, return all releases
		return releases
	}

	var unchecked []github.Release
	for _, release := range releases {
		if release.TagName == lastChecked {
			// Found the last checked release, stop here
			break
		}
		// This release is newer than last checked
		unchecked = append(unchecked, release)
	}

	return unchecked
}

// filterReleasesForBranch filters releases to only those relevant for the target branch
// e.g., "release-3.6" -> only releases starting with "v3.6"
func filterReleasesForBranch(releases []github.Release, branchName string) []github.Release {
	// Extract version from branch name
	// Expected formats: "release-3.6", "release-3.7", etc.
	version, ok := strings.CutPrefix(branchName, "release-")
	if !ok {
		// If branch doesn't match expected format, return all releases
		return releases
	}

	// Filter releases to only those starting with "v{version}"
	prefix := "v" + version
	var filtered []github.Release
	for _, release := range releases {
		if strings.HasPrefix(release.TagName, prefix) {
			filtered = append(filtered, release)
		}
	}

	return filtered
}

// isInRelease checks if a cherry-pick PR is included in any release
func isInRelease(ctx context.Context, client *github.Client, releases []github.Release, lastChecked string, originalPRNumber int) bool {
	// For each release, check if it's on the target branch
	for i := 0; i < len(releases)-1; i++ {
		currentRelease := releases[i]
		previousRelease := releases[i+1]

		// Get commits between these two releases
		commits, err := client.GetCommitsBetweenTags(ctx, previousRelease.TagName, currentRelease.TagName)
		if err != nil {
			slog.Warn("Failed to get commits between tags", "from", previousRelease.TagName, "to", currentRelease.TagName, "error", err)
			continue
		}

		// Check if any commit in this release is the cherry-pick we're looking for
		for _, commit := range commits {
			if isCherryPickCommit(commit, originalPRNumber) {
				slog.Debug("Found cherry-pick in release", "release", currentRelease.TagName, "commit", commit.SHA[:8], "original_pr", originalPRNumber)
				return true
			}
		}
	}

	// Check the oldest release (or single release case)
	// When there's only one unchecked release, we need to compare from lastChecked to it
	if len(releases) > 0 {
		oldestRelease := releases[len(releases)-1]
		// Use lastChecked as the starting point if available, otherwise start from beginning
		fromTag := lastChecked
		commits, err := client.GetCommitsBetweenTags(ctx, fromTag, oldestRelease.TagName)
		if err == nil {
			for _, commit := range commits {
				if isCherryPickCommit(commit, originalPRNumber) {
					slog.Debug("Found cherry-pick in release", "release", oldestRelease.TagName, "commit", commit.SHA[:8], "original_pr", originalPRNumber, "from", fromTag)
					return true
				}
			}
		}
	}

	return false
}

// isCherryPickCommit checks if a commit is a cherry-pick of the specified original PR
func isCherryPickCommit(commit github.Commit, originalPRNumber int) bool {
	// Check for GitHub bot cherry-pick pattern: "(cherry-pick #15033 for 3.6)"
	if matches := botCherryPickPattern.FindStringSubmatch(commit.Message); len(matches) > 1 {
		prNum, err := strconv.Atoi(matches[1])
		if err == nil && prNum == originalPRNumber {
			return true
		}
	}

	// Check for manual git cherry-pick marker: "(cherry picked from commit SHA)"
	if cherryPickPattern.MatchString(commit.Message) {
		// Also check if the commit message mentions the original PR number
		matches := prNumberPattern.FindAllStringSubmatch(commit.Message, -1)
		for _, match := range matches {
			if len(match) > 1 {
				prNum, err := strconv.Atoi(match[1])
				if err == nil && prNum == originalPRNumber {
					return true
				}
			}
		}
	}

	return false
}
