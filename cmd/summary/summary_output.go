package summary

import (
	"fmt"
	"strconv"

	"github.com/alan/cherry-picker/internal/github"
)

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
