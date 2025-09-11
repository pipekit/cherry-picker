package summary

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/alan/cherry-picker/internal/github"
	"github.com/spf13/cobra"
)

// NewSummaryCmd creates the summary command
func NewSummaryCmd(globalConfigFile *string, loadConfig func(string) (*cmd.Config, error)) *cobra.Command {
	summaryCmd := &cobra.Command{
		Use:   "summary <target-branch>",
		Short: "Generate development progress summary for a branch",
		Long: `Generate a markdown summary of development progress on the target branch since the last release.

This command shows both merged commits and work-in-progress cherry-picks from your config file.
It queries GitHub directly and uses the org/repo from the config file to show:
- [x] Completed work (merged commits and cherry-picks)
- [ ] In-progress work (picked but not yet merged)

Examples:
  cherry-picker summary release-3.7    # Dev progress for release-3.7 branch
  cherry-picker summary main           # Dev progress for main branch`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			targetBranch := args[0]
			return runSummary(*globalConfigFile, targetBranch, loadConfig)
		},
	}

	return summaryCmd
}

// runSummary executes the summary command logic
func runSummary(configFile, targetBranch string, loadConfig func(string) (*cmd.Config, error)) error {
	// Load configuration to get org and repo
	config, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	org := config.Org
	repo := config.Repo

	// Create mapping from cherry-pick PR numbers to original PR numbers
	cherryPickMap := createCherryPickMap(config, targetBranch)

	// Get GitHub token
	// Initialize GitHub client
	client, _, err := commands.InitializeGitHubClient()
	if err != nil {
		return err
	}

	fmt.Printf("🔍 Generating summary for %s/%s branch %s...\n", org, repo, targetBranch)

	// Get the last release tag for this branch
	lastTag, err := getLastReleaseTag(client, org, repo, targetBranch)
	if err != nil {
		return fmt.Errorf("failed to get last release tag: %w", err)
	}

	// Generate next version
	nextVersion, err := incrementPatchVersion(lastTag)
	if err != nil {
		return fmt.Errorf("failed to increment version: %w", err)
	}

	// Get commits since the last tag
	commits, err := getCommitsSinceTag(client, org, repo, targetBranch, lastTag)
	if err != nil {
		return fmt.Errorf("failed to get commits: %w", err)
	}

	// Get picked PRs that might not be in commits yet
	pickedPRs := getPickedPRs(config, targetBranch)

	// Get open PRs targeting this branch
	openPRs, err := client.GetOpenPRs(org, repo, targetBranch)
	if err != nil {
		return fmt.Errorf("failed to get open PRs: %w", err)
	}

	// Generate markdown output
	generateMarkdownSummary(nextVersion, lastTag, targetBranch, commits, cherryPickMap, pickedPRs, openPRs)

	return nil
}

// getLastReleaseTag finds the most recent release tag for the given branch
func getLastReleaseTag(client *github.Client, org, repo, branch string) (string, error) {
	// Get all tags from the repository
	tags, err := client.ListTags(org, repo)
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
	// Remove 'v' prefix if present
	cleanVersion := strings.TrimPrefix(version, "v")

	// Split version into parts
	parts := strings.Split(cleanVersion, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid version format: %s", version)
	}

	// Parse patch version
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid patch version: %s", parts[2])
	}

	// Increment patch version
	patch++

	// Return new version with 'v' prefix
	return fmt.Sprintf("v%s.%s.%d", parts[0], parts[1], patch), nil
}

// getCommitsSinceTag gets commits on the branch since the given tag
func getCommitsSinceTag(client *github.Client, org, repo, branch, sinceTag string) ([]github.Commit, error) {
	return client.GetCommitsSince(org, repo, branch, sinceTag)
}

// generateMarkdownSummary outputs the markdown summary
func generateMarkdownSummary(version, lastTag, branch string, commits []github.Commit, cherryPickMap map[int]int, pickedPRs []PickedPR, openPRs []github.PR) {
	if len(commits) == 0 && len(pickedPRs) == 0 && len(openPRs) == 0 {
		fmt.Printf("No changes found since %s\n", lastTag)
		return
	}

	fmt.Printf("### %s:\n\n", version)

	// Track which cherry-pick PRs we've already seen in commits
	seenCherryPickPRs := make(map[int]bool)

	// Process commits first
	for _, commit := range commits {
		if cherryPickInfo := parseCherryPickCommit(commit.Message); cherryPickInfo != nil {
			originalPR := cherryPickInfo.OriginalPR
			// If we couldn't parse the original PR from the commit message, check our mapping
			if originalPR == "unknown" {
				if cherryPickPRNum, err := strconv.Atoi(cherryPickInfo.CherryPickPR); err == nil {
					if mappedOriginal, exists := cherryPickMap[cherryPickPRNum]; exists {
						originalPR = strconv.Itoa(mappedOriginal)
					}
					// Mark this cherry-pick PR as seen
					seenCherryPickPRs[cherryPickPRNum] = true
				}
			} else {
				// Mark this cherry-pick PR as seen
				if cherryPickPRNum, err := strconv.Atoi(cherryPickInfo.CherryPickPR); err == nil {
					seenCherryPickPRs[cherryPickPRNum] = true
				}
			}
			fmt.Printf("- [x] #%s cherry-picked as #%s\n", originalPR, cherryPickInfo.CherryPickPR)
		} else {
			// Extract PR number from original commit message
			if prNumber := extractPRNumber(commit.Message); prNumber != "" {
				fmt.Printf("- [x] #%s\n", prNumber)
			} else {
				fmt.Printf("- [x] %s\n", commit.Message)
			}
		}
	}

	// Add picked PRs that haven't been seen in commits yet
	for _, pickedPR := range pickedPRs {
		if !seenCherryPickPRs[pickedPR.CherryPickPR] {
			if pickedPR.Status == "picked" {
				fmt.Printf("- [ ] #%d cherry-picked as #%d\n", pickedPR.OriginalPR, pickedPR.CherryPickPR)
			} else if pickedPR.Status == "merged" {
				fmt.Printf("- [x] #%d cherry-picked as #%d\n", pickedPR.OriginalPR, pickedPR.CherryPickPR)
			}
		}
	}

	// Add open PRs targeting this branch (these are new work, not cherry-picks)
	seenOpenPRs := make(map[int]bool)

	// First, mark any PRs we've already seen in commits or picked PRs to avoid duplicates
	for _, commit := range commits {
		if prNumber := extractPRNumber(commit.Message); prNumber != "" {
			if prNum, err := strconv.Atoi(prNumber); err == nil {
				seenOpenPRs[prNum] = true
			}
		}
	}

	// Mark picked PRs as seen
	for _, pickedPR := range pickedPRs {
		seenOpenPRs[pickedPR.CherryPickPR] = true
		seenOpenPRs[pickedPR.OriginalPR] = true
	}

	// Add open PRs that we haven't seen yet
	for _, pr := range openPRs {
		if !seenOpenPRs[pr.Number] {
			fmt.Printf("- [ ] #%d (open PR)\n", pr.Number)
		}
	}
}

// CherryPickInfo holds information about a cherry-pick commit
type CherryPickInfo struct {
	OriginalPR   string
	CherryPickPR string
}

// PickedPR represents a PR that has been picked but might not be merged yet
type PickedPR struct {
	OriginalPR   int
	CherryPickPR int
	Status       cmd.BranchStatusType // "picked" or "merged"
}

// parseCherryPickCommit parses a commit message to detect if it's a cherry-pick
// and extracts the original PR and cherry-pick PR numbers
func parseCherryPickCommit(message string) *CherryPickInfo {
	// Pattern to match cherry-pick commit messages like:
	// "some title (cherry-pick release-3.7) (#12345)"
	// The original PR would be extracted from the title or commit body

	// First, check if this looks like a cherry-pick PR title
	cherryPickPattern := regexp.MustCompile(`\(cherry-pick [^)]+\) \(#(\d+)\)$`)
	matches := cherryPickPattern.FindStringSubmatch(message)

	if len(matches) < 2 {
		return nil // Not a cherry-pick commit
	}

	cherryPickPR := matches[1]

	// Now try to extract the original PR number
	// Look for patterns like "title (#1234) (cherry-pick...)" or "title. Fixes #1234 (cherry-pick...)"
	originalPRPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\(#(\d+)\) \(cherry-pick`), // "title (#1234) (cherry-pick...)"
		regexp.MustCompile(`[Ff]ixes #(\d+)`),          // "Fixes #1234"
		regexp.MustCompile(`[Cc]loses #(\d+)`),         // "Closes #1234"
		regexp.MustCompile(`#(\d+)`),                   // Any #1234 pattern as fallback
	}

	for _, pattern := range originalPRPatterns {
		if matches := pattern.FindStringSubmatch(message); len(matches) >= 2 {
			originalPR := matches[1]
			// Make sure we didn't just capture the cherry-pick PR number
			if originalPR != cherryPickPR {
				return &CherryPickInfo{
					OriginalPR:   originalPR,
					CherryPickPR: cherryPickPR,
				}
			}
		}
	}

	// If we can't find the original PR, use "unknown"
	return &CherryPickInfo{
		OriginalPR:   "unknown",
		CherryPickPR: cherryPickPR,
	}
}

// extractPRNumber extracts a PR number from a commit message
// Looks for patterns like "title (#1234)" or "title. Fixes #1234"
func extractPRNumber(message string) string {
	// Common patterns for PR numbers in commit messages
	prPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\(#(\d+)\)$`),      // "title (#1234)" at end
		regexp.MustCompile(`[Ff]ixes #(\d+)`),  // "Fixes #1234"
		regexp.MustCompile(`[Cc]loses #(\d+)`), // "Closes #1234"
		regexp.MustCompile(`#(\d+)`),           // Any #1234 pattern as fallback
	}

	for _, pattern := range prPatterns {
		if matches := pattern.FindStringSubmatch(message); len(matches) >= 2 {
			return matches[1]
		}
	}

	return "" // No PR number found
}

// createCherryPickMap creates a mapping from cherry-pick PR numbers to original PR numbers
func createCherryPickMap(config *cmd.Config, targetBranch string) map[int]int {
	cherryPickMap := make(map[int]int)

	for _, trackedPR := range config.TrackedPRs {
		if branchStatus, exists := trackedPR.Branches[targetBranch]; exists {
			if branchStatus.PR != nil && (branchStatus.Status == cmd.BranchStatusPicked || branchStatus.Status == cmd.BranchStatusMerged) {
				// Map cherry-pick PR number -> original PR number
				cherryPickMap[branchStatus.PR.Number] = trackedPR.Number
			}
		}
	}

	return cherryPickMap
}

// getPickedPRs gets all PRs that have been picked (including merged) for the target branch
func getPickedPRs(config *cmd.Config, targetBranch string) []PickedPR {
	var pickedPRs []PickedPR

	for _, trackedPR := range config.TrackedPRs {
		if branchStatus, exists := trackedPR.Branches[targetBranch]; exists {
			if branchStatus.PR != nil && (branchStatus.Status == cmd.BranchStatusPicked || branchStatus.Status == cmd.BranchStatusMerged) {
				pickedPRs = append(pickedPRs, PickedPR{
					OriginalPR:   trackedPR.Number,
					CherryPickPR: branchStatus.PR.Number,
					Status:       branchStatus.Status,
				})
			}
		}
	}

	return pickedPRs
}

// compareVersions compares two semantic version strings
// Returns > 0 if v1 > v2, < 0 if v1 < v2, 0 if equal
func compareVersions(v1, v2 string) int {
	// Remove 'v' prefix if present
	clean1 := strings.TrimPrefix(v1, "v")
	clean2 := strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(clean1, ".")
	parts2 := strings.Split(clean2, ".")

	// Compare each part
	for i := 0; i < 3; i++ {
		if i >= len(parts1) || i >= len(parts2) {
			break
		}

		num1, _ := strconv.Atoi(parts1[i])
		num2, _ := strconv.Atoi(parts2[i])

		if num1 > num2 {
			return 1
		} else if num1 < num2 {
			return -1
		}
	}

	return 0
}
