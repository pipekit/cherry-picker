package summary

import (
	"strings"
	"testing"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/github"
)

func TestGenerateMarkdownSummary(t *testing.T) {
	tests := []struct {
		name          string
		version       string
		lastTag       string
		branch        string
		commits       []github.Commit
		cherryPickMap map[int]int
		pickedPRs     []PickedPR
		openPRs       []github.PR
		expectedLines []string
	}{
		{
			name:          "no changes",
			version:       "v3.7.1",
			lastTag:       "v3.7.0",
			branch:        "release-3.7",
			commits:       []github.Commit{},
			cherryPickMap: map[int]int{},
			pickedPRs:     []PickedPR{},
			openPRs:       []github.PR{},
			expectedLines: []string{"No changes found since v3.7.0"},
		},
		{
			name:    "single regular commit",
			version: "v3.7.1",
			lastTag: "v3.7.0",
			branch:  "release-3.7",
			commits: []github.Commit{
				{Message: "fix: some fix (#1234)"},
			},
			cherryPickMap: map[int]int{},
			pickedPRs:     []PickedPR{},
			openPRs:       []github.PR{},
			expectedLines: []string{
				"### v3.7.1:",
				"- [x] #1234",
			},
		},
		{
			name:    "cherry-pick commit",
			version: "v3.7.1",
			lastTag: "v3.7.0",
			branch:  "release-3.7",
			commits: []github.Commit{
				{Message: "fix: some fix (#1234) (cherry-pick release-3.7) (#5678)"},
			},
			cherryPickMap: map[int]int{},
			pickedPRs:     []PickedPR{},
			openPRs:       []github.PR{},
			expectedLines: []string{
				"### v3.7.1:",
				"- [x] #1234 cherry-picked as #5678",
			},
		},
		{
			name:    "cherry-pick without original PR uses map",
			version: "v3.7.1",
			lastTag: "v3.7.0",
			branch:  "release-3.7",
			commits: []github.Commit{
				{Message: "fix: some fix (cherry-pick release-3.7) (#5678)"},
			},
			cherryPickMap: map[int]int{5678: 1234},
			pickedPRs:     []PickedPR{},
			openPRs:       []github.PR{},
			expectedLines: []string{
				"### v3.7.1:",
				"- [x] #1234 cherry-picked as #5678",
			},
		},
		{
			name:          "picked PR not yet in commits",
			version:       "v3.7.1",
			lastTag:       "v3.7.0",
			branch:        "release-3.7",
			commits:       []github.Commit{},
			cherryPickMap: map[int]int{},
			pickedPRs: []PickedPR{
				{OriginalPR: 1234, CherryPickPR: 5678, Status: cmd.BranchStatusPicked},
			},
			openPRs: []github.PR{},
			expectedLines: []string{
				"### v3.7.1:",
				"- [ ] #1234 cherry-picked as #5678",
			},
		},
		{
			name:          "merged PR from picked PRs",
			version:       "v3.7.1",
			lastTag:       "v3.7.0",
			branch:        "release-3.7",
			commits:       []github.Commit{},
			cherryPickMap: map[int]int{},
			pickedPRs: []PickedPR{
				{OriginalPR: 1234, CherryPickPR: 5678, Status: cmd.BranchStatusMerged},
			},
			openPRs: []github.PR{},
			expectedLines: []string{
				"### v3.7.1:",
				"- [x] #1234 cherry-picked as #5678",
			},
		},
		{
			name:          "open PR targeting branch",
			version:       "v3.7.1",
			lastTag:       "v3.7.0",
			branch:        "release-3.7",
			commits:       []github.Commit{},
			cherryPickMap: map[int]int{},
			pickedPRs:     []PickedPR{},
			openPRs: []github.PR{
				{Number: 9999},
			},
			expectedLines: []string{
				"### v3.7.1:",
				"- [ ] #9999 (open PR)",
			},
		},
		{
			name:    "commit without PR number",
			version: "v3.7.1",
			lastTag: "v3.7.0",
			branch:  "release-3.7",
			commits: []github.Commit{
				{Message: "fix: some fix without PR"},
			},
			cherryPickMap: map[int]int{},
			pickedPRs:     []PickedPR{},
			openPRs:       []github.PR{},
			expectedLines: []string{
				"### v3.7.1:",
				"- [x] fix: some fix without PR",
			},
		},
		{
			name:    "avoid duplicate open PRs already in commits",
			version: "v3.7.1",
			lastTag: "v3.7.0",
			branch:  "release-3.7",
			commits: []github.Commit{
				{Message: "fix: some fix (#1234)"},
			},
			cherryPickMap: map[int]int{},
			pickedPRs:     []PickedPR{},
			openPRs: []github.PR{
				{Number: 1234}, // Should be filtered out since it's in commits
				{Number: 5678}, // Should be included
			},
			expectedLines: []string{
				"### v3.7.1:",
				"- [x] #1234",
				"- [ ] #5678 (open PR)",
			},
		},
		{
			name:    "avoid duplicate picked PR already in commits",
			version: "v3.7.1",
			lastTag: "v3.7.0",
			branch:  "release-3.7",
			commits: []github.Commit{
				{Message: "fix: some fix (#1234) (cherry-pick release-3.7) (#5678)"},
			},
			cherryPickMap: map[int]int{},
			pickedPRs: []PickedPR{
				{OriginalPR: 1234, CherryPickPR: 5678, Status: cmd.BranchStatusMerged},
			},
			openPRs: []github.PR{},
			expectedLines: []string{
				"### v3.7.1:",
				"- [x] #1234 cherry-picked as #5678",
			},
		},
		{
			name:    "multiple commits and PRs",
			version: "v3.7.1",
			lastTag: "v3.7.0",
			branch:  "release-3.7",
			commits: []github.Commit{
				{Message: "fix: first fix (#1111)"},
				{Message: "fix: second fix (#2222) (cherry-pick release-3.7) (#3333)"},
			},
			cherryPickMap: map[int]int{},
			pickedPRs: []PickedPR{
				{OriginalPR: 4444, CherryPickPR: 5555, Status: cmd.BranchStatusPicked},
			},
			openPRs: []github.PR{
				{Number: 6666},
			},
			expectedLines: []string{
				"### v3.7.1:",
				"- [x] #1111",
				"- [x] #2222 cherry-picked as #3333",
				"- [ ] #4444 cherry-picked as #5555",
				"- [ ] #6666 (open PR)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := generateMarkdownSummary(tt.version, tt.lastTag, tt.branch, tt.commits, tt.cherryPickMap, tt.pickedPRs, tt.openPRs)

			// Check that all expected lines are present
			for _, expectedLine := range tt.expectedLines {
				if !strings.Contains(output, expectedLine) {
					t.Errorf("generateMarkdownSummary() output missing expected line: %q\nGot output:\n%s", expectedLine, output)
				}
			}
		})
	}
}

func TestGenerateMarkdownSummaryFormat(t *testing.T) {
	t.Run("output starts with version header", func(t *testing.T) {
		commits := []github.Commit{
			{Message: "fix: some fix (#1234)"},
		}

		output := generateMarkdownSummary("v3.7.1", "v3.7.0", "release-3.7", commits, map[int]int{}, []PickedPR{}, []github.PR{})

		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) < 1 {
			t.Fatal("generateMarkdownSummary() produced no output")
		}

		if !strings.HasPrefix(lines[0], "### v3.7.1:") {
			t.Errorf("generateMarkdownSummary() first line = %q, want to start with '### v3.7.1:'", lines[0])
		}
	})

	t.Run("completed items use [x]", func(t *testing.T) {
		commits := []github.Commit{
			{Message: "fix: some fix (#1234)"},
		}

		output := generateMarkdownSummary("v3.7.1", "v3.7.0", "release-3.7", commits, map[int]int{}, []PickedPR{}, []github.PR{})

		if !strings.Contains(output, "- [x]") {
			t.Error("generateMarkdownSummary() completed items should use '- [x]'")
		}
	})

	t.Run("in-progress items use [ ]", func(t *testing.T) {
		pickedPRs := []PickedPR{
			{OriginalPR: 1234, CherryPickPR: 5678, Status: cmd.BranchStatusPicked},
		}

		output := generateMarkdownSummary("v3.7.1", "v3.7.0", "release-3.7", []github.Commit{}, map[int]int{}, pickedPRs, []github.PR{})

		if !strings.Contains(output, "- [ ]") {
			t.Error("generateMarkdownSummary() in-progress items should use '- [ ]'")
		}
	})
}
