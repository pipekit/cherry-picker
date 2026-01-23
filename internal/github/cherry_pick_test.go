package github

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCherryPickSuccessPattern(t *testing.T) {
	// Test the regex pattern used in GetCherryPickPRsFromComments
	successPattern := regexp.MustCompile(`Cherry-pick PR created for ([0-9.]+): #(\d+)`)

	tests := []struct {
		name           string
		comment        string
		shouldMatch    bool
		expectedBranch string
		expectedPRNum  string
	}{
		{
			name:           "standard success message",
			comment:        "🍒 Cherry-pick PR created for 3.7: #14944",
			shouldMatch:    true,
			expectedBranch: "3.7",
			expectedPRNum:  "14944",
		},
		{
			name:           "without emoji",
			comment:        "Cherry-pick PR created for 3.6: #123",
			shouldMatch:    true,
			expectedBranch: "3.6",
			expectedPRNum:  "123",
		},
		{
			name:           "with extra text before",
			comment:        "Success! Cherry-pick PR created for 4.0: #999",
			shouldMatch:    true,
			expectedBranch: "4.0",
			expectedPRNum:  "999",
		},
		{
			name:           "multiple version parts",
			comment:        "Cherry-pick PR created for 3.7.1: #555",
			shouldMatch:    true,
			expectedBranch: "3.7.1",
			expectedPRNum:  "555",
		},
		{
			name:        "failure message - no match",
			comment:     "❌ Cherry-pick failed for 3.7.",
			shouldMatch: false,
		},
		{
			name:        "wrong format - no match",
			comment:     "Created PR #123 for cherry-pick",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := successPattern.FindStringSubmatch(tt.comment)

			if tt.shouldMatch {
				assert.NotNil(t, matches, "should match pattern")
				assert.Len(t, matches, 3, "should have branch and PR number captures")
				assert.Equal(t, tt.expectedBranch, matches[1], "branch version")
				assert.Equal(t, tt.expectedPRNum, matches[2], "PR number")
			} else {
				assert.Nil(t, matches, "should not match pattern")
			}
		})
	}
}

func TestCherryPickFailurePattern(t *testing.T) {
	// Test the regex pattern used in GetCherryPickPRsFromComments
	failurePattern := regexp.MustCompile(`Cherry-pick failed for ([0-9.]+)\.`)

	tests := []struct {
		name           string
		comment        string
		shouldMatch    bool
		expectedBranch string
	}{
		{
			name:           "standard failure message",
			comment:        "❌ Cherry-pick failed for 3.7.",
			shouldMatch:    true,
			expectedBranch: "3.7",
		},
		{
			name:           "without emoji",
			comment:        "Cherry-pick failed for 3.6.",
			shouldMatch:    true,
			expectedBranch: "3.6",
		},
		{
			name:           "with extra text after",
			comment:        "Cherry-pick failed for 4.0. Please resolve conflicts manually.",
			shouldMatch:    true,
			expectedBranch: "4.0",
		},
		{
			name:           "multiple version parts",
			comment:        "Cherry-pick failed for 3.7.1.",
			shouldMatch:    true,
			expectedBranch: "3.7.1",
		},
		{
			name:        "success message - no match",
			comment:     "Cherry-pick PR created for 3.7: #123",
			shouldMatch: false,
		},
		{
			name:        "missing trailing period - no match",
			comment:     "Cherry-pick failed for 37",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := failurePattern.FindStringSubmatch(tt.comment)

			if tt.shouldMatch {
				assert.NotNil(t, matches, "should match pattern")
				assert.Len(t, matches, 2, "should have branch capture")
				assert.Equal(t, tt.expectedBranch, matches[1], "branch version")
			} else {
				assert.Nil(t, matches, "should not match pattern")
			}
		})
	}
}

func TestManualCherryPickTitlePattern(t *testing.T) {
	// Test the regex pattern used in SearchManualCherryPickPRs
	// Pattern matches titles containing "cherry-pick" followed by optional # and a number
	pattern := regexp.MustCompile(`(?i)cherry-pick\s+#?\d+`)

	tests := []struct {
		name        string
		title       string
		shouldMatch bool
	}{
		{
			name:        "standard format with hash",
			title:       "cherry-pick #14894: Fix bug",
			shouldMatch: true,
		},
		{
			name:        "without hash",
			title:       "cherry-pick 14894: Fix bug",
			shouldMatch: true,
		},
		{
			name:        "in parentheses",
			title:       "Fix bug (cherry-pick #14894)",
			shouldMatch: true,
		},
		{
			name:        "case insensitive - uppercase",
			title:       "Cherry-Pick #123",
			shouldMatch: true,
		},
		{
			name:        "case insensitive - mixed case",
			title:       "CHERRY-PICK #456",
			shouldMatch: true,
		},
		{
			name:        "no PR number - no match",
			title:       "cherry-pick: Fix bug",
			shouldMatch: false,
		},
		{
			name:        "just the word cherry - no match",
			title:       "Cherry flavored fix",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := pattern.MatchString(tt.title)
			assert.Equal(t, tt.shouldMatch, matches, "title: %s", tt.title)
		})
	}
}

func TestManualCherryPickTitleWithBranchPattern(t *testing.T) {
	// Test the enhanced pattern that extracts branch from title
	// Pattern matches "(cherry-pick #14894 for 3.7)" and extracts the version
	prNumber := 15061
	pattern := regexp.MustCompile(fmt.Sprintf(`(?i)cherry-pick\s+#?%d\s+for\s+([0-9.]+)`, prNumber))

	tests := []struct {
		name            string
		title           string
		shouldMatch     bool
		expectedVersion string
	}{
		{
			name:            "standard format with hash and parentheses",
			title:           "fix: some bug (cherry-pick #15061 for 3.7)",
			shouldMatch:     true,
			expectedVersion: "3.7",
		},
		{
			name:            "without parentheses",
			title:           "cherry-pick #15061 for 3.6",
			shouldMatch:     true,
			expectedVersion: "3.6",
		},
		{
			name:            "complex title with multiple cherry-picks",
			title:           "chore(otel): add support (cherry-pick #15061 for 3.7)(cherry-pick #15067 for 3.7)",
			shouldMatch:     true,
			expectedVersion: "3.7",
		},
		{
			name:            "case insensitive",
			title:           "Fix bug (Cherry-Pick #15061 for 4.0)",
			shouldMatch:     true,
			expectedVersion: "4.0",
		},
		{
			name:            "version with patch number",
			title:           "fix: bug (cherry-pick #15061 for 3.7.1)",
			shouldMatch:     true,
			expectedVersion: "3.7.1",
		},
		{
			name:        "different PR number - no match",
			title:       "fix: bug (cherry-pick #12345 for 3.7)",
			shouldMatch: false,
		},
		{
			name:        "missing for clause - no match",
			title:       "fix: bug (cherry-pick #15061)",
			shouldMatch: false,
		},
		{
			name:        "missing version - no match",
			title:       "cherry-pick #15061 for release",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := pattern.FindStringSubmatch(tt.title)

			if tt.shouldMatch {
				assert.NotNil(t, matches, "should match pattern")
				assert.Len(t, matches, 2, "should have version capture")
				assert.Equal(t, tt.expectedVersion, matches[1], "version")
			} else {
				assert.Nil(t, matches, "should not match pattern")
			}
		})
	}
}

func TestCherryPickPRPriorityMerging(t *testing.T) {
	// Test the priority logic for merging cherry-pick PR results
	// Non-failures should always win over failures
	// When both are successes, higher PR number (newer) wins

	tests := []struct {
		name           string
		entries        []CherryPickPR
		expectedNumber int
		expectedFailed bool
	}{
		{
			name: "failure then success - success wins",
			entries: []CherryPickPR{
				{Number: 0, Branch: "release-3.7", Failed: true},
				{Number: 15083, Branch: "release-3.7", Failed: false},
			},
			expectedNumber: 15083,
			expectedFailed: false,
		},
		{
			name: "success then failure - success preserved",
			entries: []CherryPickPR{
				{Number: 15083, Branch: "release-3.7", Failed: false},
				{Number: 0, Branch: "release-3.7", Failed: true},
			},
			expectedNumber: 15083,
			expectedFailed: false,
		},
		{
			name: "two successes - higher PR number wins",
			entries: []CherryPickPR{
				{Number: 15000, Branch: "release-3.7", Failed: false},
				{Number: 15083, Branch: "release-3.7", Failed: false},
			},
			expectedNumber: 15083,
			expectedFailed: false,
		},
		{
			name: "two successes reverse order - higher PR number wins",
			entries: []CherryPickPR{
				{Number: 15083, Branch: "release-3.7", Failed: false},
				{Number: 15000, Branch: "release-3.7", Failed: false},
			},
			expectedNumber: 15083,
			expectedFailed: false,
		},
		{
			name: "only failure",
			entries: []CherryPickPR{
				{Number: 0, Branch: "release-3.7", Failed: true},
			},
			expectedNumber: 0,
			expectedFailed: true,
		},
		{
			name: "multiple failures then success",
			entries: []CherryPickPR{
				{Number: 0, Branch: "release-3.7", Failed: true},
				{Number: 0, Branch: "release-3.7", Failed: true},
				{Number: 15083, Branch: "release-3.7", Failed: false},
			},
			expectedNumber: 15083,
			expectedFailed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the priority merging logic from fetch_tracking.go
			existingByBranch := make(map[string]CherryPickPR)
			for _, cp := range tt.entries {
				existing, exists := existingByBranch[cp.Branch]
				if !exists {
					existingByBranch[cp.Branch] = cp
				} else if existing.Failed && !cp.Failed {
					// Non-failure always wins over failure
					existingByBranch[cp.Branch] = cp
				} else if !existing.Failed && !cp.Failed && cp.Number > existing.Number {
					// Both successes: prefer higher PR number (newer)
					existingByBranch[cp.Branch] = cp
				}
			}

			result := existingByBranch["release-3.7"]
			assert.Equal(t, tt.expectedNumber, result.Number, "PR number")
			assert.Equal(t, tt.expectedFailed, result.Failed, "failed status")
		})
	}
}
