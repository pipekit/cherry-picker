package github

import (
	"regexp"
	"strconv"
)

// Cherry-pick detection patterns - shared between PR title matching and commit message matching

// cherryPickContextPattern detects if text contains cherry-pick context
var cherryPickContextPattern = regexp.MustCompile(`(?i)cherry[- ]?pick`)

// Bot comment patterns for detecting cherry-pick status from bot comments
// BotSuccessPattern matches: "🍒 Cherry-pick PR created for 3.7: #14944"
var BotSuccessPattern = regexp.MustCompile(`Cherry-pick PR created for ([0-9.]+): #(\d+)`)

// BotFailurePattern matches: "❌ Cherry-pick failed for 3.7."
var BotFailurePattern = regexp.MustCompile(`Cherry-pick failed for ([0-9.]+)\.`)

// botCherryPickPattern matches GitHub bot cherry-pick format with version
// Example: "(cherry-pick #15033 for 3.6)" -> extracts PR number and version
var botCherryPickPattern = regexp.MustCompile(`(?i)\(cherry-pick\s+#?(\d+)\s+for\s+([0-9.]+)\)`)

// prNumberPattern matches PR numbers with # prefix
var prNumberPattern = regexp.MustCompile(`#(\d+)`)

// prNumberInContextPattern matches PR numbers with or without # prefix (4+ digits to avoid false positives)
var prNumberInContextPattern = regexp.MustCompile(`\b(\d{4,})\b`)

// gitCherryPickPattern matches the line added by 'git cherry-pick -x'
// Example: "(cherry picked from commit abc123def456)"
var gitCherryPickPattern = regexp.MustCompile(`\(cherry picked from commit ([a-f0-9]+)\)`)

// CherryPickMatch represents a detected cherry-pick reference
type CherryPickMatch struct {
	PRNumber int
	Branch   string // e.g., "release-3.7" (empty if not detected)
}

// ExtractCherryPickPRs extracts cherry-pick PR references from text (PR title or commit message)
// Returns all PR numbers found in cherry-pick context
func ExtractCherryPickPRs(text string) []CherryPickMatch {
	var matches []CherryPickMatch
	seen := make(map[int]bool)

	// First, try bot pattern which includes version: "(cherry-pick #15033 for 3.6)"
	botMatches := botCherryPickPattern.FindAllStringSubmatch(text, -1)
	for _, match := range botMatches {
		if len(match) >= 3 {
			prNum, err := strconv.Atoi(match[1])
			if err == nil && !seen[prNum] {
				seen[prNum] = true
				matches = append(matches, CherryPickMatch{
					PRNumber: prNum,
					Branch:   "release-" + match[2],
				})
			}
		}
	}

	// If text contains cherry-pick context, extract all PR numbers
	if cherryPickContextPattern.MatchString(text) {
		// Extract PR numbers with or without # prefix
		prMatches := prNumberInContextPattern.FindAllStringSubmatch(text, -1)
		for _, match := range prMatches {
			if len(match) >= 2 {
				prNum, err := strconv.Atoi(match[1])
				if err == nil && !seen[prNum] {
					seen[prNum] = true
					matches = append(matches, CherryPickMatch{
						PRNumber: prNum,
						Branch:   "", // Branch not determined from this pattern
					})
				}
			}
		}
	}

	return matches
}

// ContainsCherryPickForPR checks if text contains a cherry-pick reference for the specified PR number
func ContainsCherryPickForPR(text string, prNumber int) bool {
	// Quick check: does the text contain the PR number at all?
	if !prNumberInContextPattern.MatchString(text) {
		return false
	}

	// Check bot pattern
	botMatches := botCherryPickPattern.FindAllStringSubmatch(text, -1)
	for _, match := range botMatches {
		if len(match) >= 2 {
			prNum, err := strconv.Atoi(match[1])
			if err == nil && prNum == prNumber {
				return true
			}
		}
	}

	// Check git cherry-pick marker with PR reference
	if gitCherryPickPattern.MatchString(text) {
		prMatches := prNumberPattern.FindAllStringSubmatch(text, -1)
		for _, match := range prMatches {
			if len(match) >= 2 {
				prNum, err := strconv.Atoi(match[1])
				if err == nil && prNum == prNumber {
					return true
				}
			}
		}
	}

	// Check if text contains cherry-pick context and the PR number
	if cherryPickContextPattern.MatchString(text) {
		prMatches := prNumberInContextPattern.FindAllStringSubmatch(text, -1)
		for _, match := range prMatches {
			if len(match) >= 2 {
				prNum, err := strconv.Atoi(match[1])
				if err == nil && prNum == prNumber {
					return true
				}
			}
		}
	}

	return false
}

// ExtractBranchFromCherryPickTitle extracts the target branch from a cherry-pick title/message
// Returns the branch name (e.g., "release-3.7") and whether it was found
func ExtractBranchFromCherryPickTitle(text string, prNumber int) (string, bool) {
	// Try bot pattern first: "(cherry-pick #15033 for 3.6)"
	matches := botCherryPickPattern.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			prNum, err := strconv.Atoi(match[1])
			if err == nil && prNum == prNumber {
				return "release-" + match[2], true
			}
		}
	}
	return "", false
}
