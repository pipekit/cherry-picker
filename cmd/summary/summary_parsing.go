package summary

import (
	"regexp"

	"github.com/alan/cherry-picker/cmd"
)

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
