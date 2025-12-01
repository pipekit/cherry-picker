package summary

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/github"
)

// generateMarkdownSummary returns the markdown summary as a string
func generateMarkdownSummary(version, lastTag, _ string, commits []github.Commit, cherryPickMap map[int]int, pickedPRs []PickedPR) string {
	if len(commits) == 0 && len(pickedPRs) == 0 {
		return fmt.Sprintf("No changes found since %s\n", lastTag)
	}

	type entry struct {
		prNum int
		line  string
	}
	var entries []entry
	seenCherryPickPRs := make(map[int]bool)

	// Process commits
	for _, commit := range commits {
		if cherryPickInfo := parseCherryPickCommit(commit.Message); cherryPickInfo != nil {
			originalPR := cherryPickInfo.OriginalPR
			if cherryPickPRNum, err := strconv.Atoi(cherryPickInfo.CherryPickPR); err == nil {
				if mappedOriginal, exists := cherryPickMap[cherryPickPRNum]; exists {
					originalPR = strconv.Itoa(mappedOriginal)
				}
				seenCherryPickPRs[cherryPickPRNum] = true
			}
			prNum, _ := strconv.Atoi(originalPR)
			entries = append(entries, entry{prNum, fmt.Sprintf("- [x] #%s cherry-picked as #%s\n", originalPR, cherryPickInfo.CherryPickPR)})
		} else if prNumber := extractPRNumber(commit.Message); prNumber != "" {
			prNum, _ := strconv.Atoi(prNumber)
			entries = append(entries, entry{prNum, fmt.Sprintf("- [x] #%s\n", prNumber)})
		} else {
			entries = append(entries, entry{0, fmt.Sprintf("- [x] %s\n", commit.Message)})
		}
	}

	// Add picked PRs not yet in commits
	for _, pickedPR := range pickedPRs {
		if !seenCherryPickPRs[pickedPR.CherryPickPR] {
			var line string
			switch pickedPR.Status {
			case cmd.BranchStatusPending:
				line = fmt.Sprintf("- [ ] #%d\n", pickedPR.OriginalPR)
			case cmd.BranchStatusPicked:
				fallthrough
			case cmd.BranchStatusFailed:
				line = fmt.Sprintf("- [ ] #%d cherry-picked as #%d\n", pickedPR.OriginalPR, pickedPR.CherryPickPR)
			case cmd.BranchStatusMerged:
				line = fmt.Sprintf("- [x] #%d cherry-picked as #%d\n", pickedPR.OriginalPR, pickedPR.CherryPickPR)
			case cmd.BranchStatusReleased:
				line = ""
			}
			if line != "" {
				entries = append(entries, entry{pickedPR.OriginalPR, line})
			}
		}
	}

	// Sort by PR number (0s go last)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].prNum == 0 {
			return false
		}
		if entries[j].prNum == 0 {
			return true
		}
		return entries[i].prNum < entries[j].prNum
	})

	var output strings.Builder
	output.WriteString(fmt.Sprintf("### %s:\n\n", version))
	for _, e := range entries {
		output.WriteString(e.line)
	}
	return output.String()
}
