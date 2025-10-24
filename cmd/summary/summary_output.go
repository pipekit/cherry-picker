package summary

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/github"
)

// generateMarkdownSummary returns the markdown summary as a string
func generateMarkdownSummary(version, lastTag, _ string, commits []github.Commit, cherryPickMap map[int]int, pickedPRs []PickedPR, openPRs []github.PR) string {
	if len(commits) == 0 && len(pickedPRs) == 0 && len(openPRs) == 0 {
		return fmt.Sprintf("No changes found since %s\n", lastTag)
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("### %s:\n\n", version))

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
			output.WriteString(fmt.Sprintf("- [x] #%s cherry-picked as #%s\n", originalPR, cherryPickInfo.CherryPickPR))
		} else {
			// Extract PR number from original commit message
			if prNumber := extractPRNumber(commit.Message); prNumber != "" {
				output.WriteString(fmt.Sprintf("- [x] #%s\n", prNumber))
			} else {
				output.WriteString(fmt.Sprintf("- [x] %s\n", commit.Message))
			}
		}
	}

	// Add picked PRs that haven't been seen in commits yet
	for _, pickedPR := range pickedPRs {
		if !seenCherryPickPRs[pickedPR.CherryPickPR] {
			switch pickedPR.Status {
			case cmd.BranchStatusPicked:
				output.WriteString(fmt.Sprintf("- [ ] #%d cherry-picked as #%d\n", pickedPR.OriginalPR, pickedPR.CherryPickPR))
			case cmd.BranchStatusMerged:
				output.WriteString(fmt.Sprintf("- [x] #%d cherry-picked as #%d\n", pickedPR.OriginalPR, pickedPR.CherryPickPR))
			case cmd.BranchStatusReleased:
				output.WriteString(fmt.Sprintf("- [x] #%d cherry-picked as #%d (released)\n", pickedPR.OriginalPR, pickedPR.CherryPickPR))
			case cmd.BranchStatusPending, cmd.BranchStatusFailed:
				// These statuses shouldn't appear in picked PRs, but handle them for exhaustiveness
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
			output.WriteString(fmt.Sprintf("- [ ] #%d (open PR)\n", pr.Number))
		}
	}

	return output.String()
}
