package github

import (
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
