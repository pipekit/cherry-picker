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

	// Track which branches we've checked (to update last checked release)
	branchesChecked := make(map[string]string) // branch -> latest release checked

	// For each tracked PR, check each branch that's merged
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

			// Filter releases to only those relevant for this branch
			// e.g., "release-3.6" -> only check releases starting with "v3.6"
			relevantReleases := filterReleasesForBranch(allReleases, branchName)
			if len(relevantReleases) == 0 {
				slog.Debug("No relevant releases for branch", "branch", branchName)
				continue
			}

			// Filter to only unchecked releases (newer than last checked)
			lastChecked := config.LastCheckedRelease[branchName]
			uncheckedReleases := filterUncheckedReleases(relevantReleases, lastChecked)

			if len(uncheckedReleases) == 0 {
				slog.Debug("No new releases to check for branch", "branch", branchName, "last_checked", lastChecked)
				continue
			}

			slog.Debug("Checking new releases", "branch", branchName, "new_count", len(uncheckedReleases), "total_relevant", len(relevantReleases))

			// Track the latest release we're checking for this branch
			if len(uncheckedReleases) > 0 {
				// Releases are sorted newest first, so first one is the latest
				branchesChecked[branchName] = uncheckedReleases[0].TagName
			}

			// Check if this cherry-pick PR's commit is in any release
			if isInRelease(ctx, client, uncheckedReleases, trackedPR.Number) {
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
func isInRelease(ctx context.Context, client *github.Client, releases []github.Release, originalPRNumber int) bool {
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

	// Check the oldest release (between first release and beginning of time)
	if len(releases) > 0 {
		oldestRelease := releases[len(releases)-1]
		commits, err := client.GetCommitsBetweenTags(ctx, "", oldestRelease.TagName)
		if err == nil {
			for _, commit := range commits {
				if isCherryPickCommit(commit, originalPRNumber) {
					slog.Debug("Found cherry-pick in oldest release", "release", oldestRelease.TagName, "commit", commit.SHA[:8], "original_pr", originalPRNumber)
					return true
				}
			}
		}
	}

	return false
}

// isCherryPickCommit checks if a commit is a cherry-pick of the specified original PR
func isCherryPickCommit(commit github.Commit, originalPRNumber int) bool {
	// Check for cherry-pick marker
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
