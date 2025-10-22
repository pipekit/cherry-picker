package summary

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/alan/cherry-picker/internal/github"
)

// getLastReleaseTag finds the most recent release tag for the given branch
func getLastReleaseTag(client *github.Client, branch string) (string, error) {
	// Get all tags from the repository
	tags, err := client.ListTags()
	if err != nil {
		return "", fmt.Errorf("failed to list tags: %w", err)
	}

	if len(tags) == 0 {
		// No tags found, assume this is the first release
		return "v0.0.0", nil
	}

	// Extract version prefix from branch name (e.g., "release-3.6" -> "3.6")
	var versionPrefix string
	if strings.HasPrefix(branch, "release-") {
		versionPrefix = strings.TrimPrefix(branch, "release-")
	} else {
		// For non-release branches, use the branch name as-is
		versionPrefix = branch
	}

	// Filter tags that match the branch version prefix
	var validTags []string
	semverPattern := regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)

	for _, tag := range tags {
		if semverPattern.MatchString(tag) {
			// Check if tag matches the branch version prefix
			cleanTag := strings.TrimPrefix(tag, "v")
			parts := strings.Split(cleanTag, ".")
			if len(parts) >= 2 && parts[0]+"."+parts[1] == versionPrefix {
				validTags = append(validTags, tag)
			}
		}
	}

	if len(validTags) == 0 {
		// No matching tags found for this branch, assume this is the first release
		return fmt.Sprintf("v%s.0", versionPrefix), nil
	}

	// Sort tags in descending order (most recent first)
	sort.Slice(validTags, func(i, j int) bool {
		return compareVersions(validTags[i], validTags[j]) > 0
	})

	return validTags[0], nil
}

// incrementPatchVersion takes a version string and increments the patch version
func incrementPatchVersion(version string) (string, error) {
	v, err := semver.NewVersion(version)
	if err != nil {
		return "", fmt.Errorf("invalid version format: %s", version)
	}

	// Increment patch version
	next := v.IncPatch()

	// Return new version with 'v' prefix
	return fmt.Sprintf("v%s", next.String()), nil
}

// compareVersions compares two semantic version strings
// Returns > 0 if v1 > v2, < 0 if v1 < v2, 0 if equal
// If parsing fails, returns 0 (versions are considered equal)
func compareVersions(v1, v2 string) int {
	ver1, err1 := semver.NewVersion(v1)
	ver2, err2 := semver.NewVersion(v2)

	// If either version is invalid, treat as equal (defensive)
	if err1 != nil || err2 != nil {
		return 0
	}

	return ver1.Compare(ver2)
}

// getCommitsSinceTag gets commits on the branch since the given tag
func getCommitsSinceTag(client *github.Client, branch, sinceTag string) ([]github.Commit, error) {
	return client.GetCommitsSince(branch, sinceTag)
}
