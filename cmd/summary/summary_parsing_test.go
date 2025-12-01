package summary

import (
	"testing"

	"github.com/alan/cherry-picker/cmd"
)

func TestParseCherryPickCommit(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected *CherryPickInfo
	}{
		{
			name:     "not a cherry-pick commit",
			message:  "fix: some regular commit (#1234)",
			expected: nil,
		},
		{
			name:     "cherry-pick with original PR before",
			message:  "fix: some fix (#1234) (cherry-pick release-3.7) (#5678)",
			expected: &CherryPickInfo{OriginalPR: "1234", CherryPickPR: "5678"},
		},
		{
			name:     "cherry-pick with Fixes pattern",
			message:  "fix: some fix. Fixes #1234 (cherry-pick release-3.7) (#5678)",
			expected: &CherryPickInfo{OriginalPR: "1234", CherryPickPR: "5678"},
		},
		{
			name:     "cherry-pick with Closes pattern",
			message:  "fix: some fix. Closes #1234 (cherry-pick release-3.7) (#5678)",
			expected: &CherryPickInfo{OriginalPR: "1234", CherryPickPR: "5678"},
		},
		{
			name:     "cherry-pick with lowercase fixes",
			message:  "fix: some fix. fixes #1234 (cherry-pick release-3.7) (#5678)",
			expected: &CherryPickInfo{OriginalPR: "1234", CherryPickPR: "5678"},
		},
		{
			name:     "cherry-pick with lowercase closes",
			message:  "fix: some fix. closes #1234 (cherry-pick release-3.7) (#5678)",
			expected: &CherryPickInfo{OriginalPR: "1234", CherryPickPR: "5678"},
		},
		{
			name:     "cherry-pick without original PR - unknown",
			message:  "fix: some fix (cherry-pick release-3.7) (#5678)",
			expected: &CherryPickInfo{OriginalPR: "unknown", CherryPickPR: "5678"},
		},
		{
			name:     "cherry-pick with multiple # signs - uses first non-cherry-pick",
			message:  "fix: some fix (#1234) mentions #999 (cherry-pick release-3.7) (#5678)",
			expected: &CherryPickInfo{OriginalPR: "1234", CherryPickPR: "5678"},
		},
		{
			name:     "cherry-pick with only fallback # pattern",
			message:  "fix: some fix #1234 text (cherry-pick release-3.7) (#5678)",
			expected: &CherryPickInfo{OriginalPR: "1234", CherryPickPR: "5678"},
		},
		{
			name:     "not a cherry-pick - missing closing paren",
			message:  "fix: some fix (cherry-pick release-3.7 (#5678)",
			expected: nil,
		},
		{
			name:     "cherry-pick different branch name",
			message:  "fix: some fix (#1234) (cherry-pick main) (#5678)",
			expected: &CherryPickInfo{OriginalPR: "1234", CherryPickPR: "5678"},
		},
		{
			name:     "cherry-pick with PR number in cherry-pick reference",
			message:  "fix: cache calls to prevent exponential recursion. Fixes #14904 (cherry-pick #14920 for 3.7) (#14988)",
			expected: &CherryPickInfo{OriginalPR: "14920", CherryPickPR: "14988"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCherryPickCommit(tt.message)

			if tt.expected == nil {
				if got != nil {
					t.Errorf("parseCherryPickCommit() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Errorf("parseCherryPickCommit() = nil, want %v", tt.expected)
				return
			}

			if got.OriginalPR != tt.expected.OriginalPR || got.CherryPickPR != tt.expected.CherryPickPR {
				t.Errorf("parseCherryPickCommit() = {OriginalPR: %v, CherryPickPR: %v}, want {OriginalPR: %v, CherryPickPR: %v}",
					got.OriginalPR, got.CherryPickPR, tt.expected.OriginalPR, tt.expected.CherryPickPR)
			}
		})
	}
}

func TestExtractPRNumber(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "PR number at end with parens",
			message:  "fix: some commit message (#1234)",
			expected: "1234",
		},
		{
			name:     "PR number with Fixes keyword",
			message:  "fix: some commit. Fixes #5678",
			expected: "5678",
		},
		{
			name:     "PR number with fixes keyword (lowercase)",
			message:  "fix: some commit. fixes #5678",
			expected: "5678",
		},
		{
			name:     "PR number with Closes keyword",
			message:  "fix: some commit. Closes #9999",
			expected: "9999",
		},
		{
			name:     "PR number with closes keyword (lowercase)",
			message:  "fix: some commit. closes #9999",
			expected: "9999",
		},
		{
			name:     "PR number anywhere in message - fallback",
			message:  "fix: some commit related to #4321",
			expected: "4321",
		},
		{
			name:     "no PR number",
			message:  "fix: some commit without PR reference",
			expected: "",
		},
		{
			name:     "multiple PR numbers - returns first match",
			message:  "fix: some commit (#1234) also #5678",
			expected: "1234",
		},
		{
			name:     "PR number not at end",
			message:  "fix: (#1234) some commit",
			expected: "1234",
		},
		{
			name:     "empty message",
			message:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPRNumber(tt.message)
			if got != tt.expected {
				t.Errorf("extractPRNumber() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCreateCherryPickMap(t *testing.T) {
	tests := []struct {
		name         string
		config       *cmd.Config
		targetBranch string
		expected     map[int]int
	}{
		{
			name: "empty config",
			config: &cmd.Config{
				TrackedPRs: []cmd.TrackedPR{},
			},
			targetBranch: "release-3.7",
			expected:     map[int]int{},
		},
		{
			name: "single picked PR",
			config: &cmd.Config{
				TrackedPRs: []cmd.TrackedPR{
					{
						Number: 1234,
						Branches: map[string]cmd.BranchStatus{
							"release-3.7": {
								Status: cmd.BranchStatusPicked,
								PR: &cmd.PickPR{
									Number: 5678,
								},
							},
						},
					},
				},
			},
			targetBranch: "release-3.7",
			expected:     map[int]int{5678: 1234},
		},
		{
			name: "single merged PR",
			config: &cmd.Config{
				TrackedPRs: []cmd.TrackedPR{
					{
						Number: 1234,
						Branches: map[string]cmd.BranchStatus{
							"release-3.7": {
								Status: cmd.BranchStatusMerged,
								PR: &cmd.PickPR{
									Number: 5678,
								},
							},
						},
					},
				},
			},
			targetBranch: "release-3.7",
			expected:     map[int]int{5678: 1234},
		},
		{
			name: "multiple PRs",
			config: &cmd.Config{
				TrackedPRs: []cmd.TrackedPR{
					{
						Number: 1234,
						Branches: map[string]cmd.BranchStatus{
							"release-3.7": {
								Status: cmd.BranchStatusPicked,
								PR: &cmd.PickPR{
									Number: 5678,
								},
							},
						},
					},
					{
						Number: 2345,
						Branches: map[string]cmd.BranchStatus{
							"release-3.7": {
								Status: cmd.BranchStatusMerged,
								PR: &cmd.PickPR{
									Number: 6789,
								},
							},
						},
					},
				},
			},
			targetBranch: "release-3.7",
			expected:     map[int]int{5678: 1234, 6789: 2345},
		},
		{
			name: "PR with pending status not included",
			config: &cmd.Config{
				TrackedPRs: []cmd.TrackedPR{
					{
						Number: 1234,
						Branches: map[string]cmd.BranchStatus{
							"release-3.7": {
								Status: cmd.BranchStatusPending,
							},
						},
					},
				},
			},
			targetBranch: "release-3.7",
			expected:     map[int]int{},
		},
		{
			name: "PR with failed status not included",
			config: &cmd.Config{
				TrackedPRs: []cmd.TrackedPR{
					{
						Number: 1234,
						Branches: map[string]cmd.BranchStatus{
							"release-3.7": {
								Status: cmd.BranchStatusFailed,
							},
						},
					},
				},
			},
			targetBranch: "release-3.7",
			expected:     map[int]int{},
		},
		{
			name: "different target branch not included",
			config: &cmd.Config{
				TrackedPRs: []cmd.TrackedPR{
					{
						Number: 1234,
						Branches: map[string]cmd.BranchStatus{
							"release-3.6": {
								Status: cmd.BranchStatusPicked,
								PR: &cmd.PickPR{
									Number: 5678,
								},
							},
						},
					},
				},
			},
			targetBranch: "release-3.7",
			expected:     map[int]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createCherryPickMap(tt.config, tt.targetBranch)

			if len(got) != len(tt.expected) {
				t.Errorf("createCherryPickMap() returned map with %d entries, want %d", len(got), len(tt.expected))
				return
			}

			for k, v := range tt.expected {
				if gotV, exists := got[k]; !exists || gotV != v {
					t.Errorf("createCherryPickMap()[%d] = %v, want %v", k, gotV, v)
				}
			}
		})
	}
}

func TestGetPickedPRs(t *testing.T) {
	tests := []struct {
		name         string
		config       *cmd.Config
		targetBranch string
		expected     []PickedPR
	}{
		{
			name: "empty config",
			config: &cmd.Config{
				TrackedPRs: []cmd.TrackedPR{},
			},
			targetBranch: "release-3.7",
			expected:     nil,
		},
		{
			name: "single picked PR",
			config: &cmd.Config{
				TrackedPRs: []cmd.TrackedPR{
					{
						Number: 1234,
						Branches: map[string]cmd.BranchStatus{
							"release-3.7": {
								Status: cmd.BranchStatusPicked,
								PR: &cmd.PickPR{
									Number: 5678,
								},
							},
						},
					},
				},
			},
			targetBranch: "release-3.7",
			expected: []PickedPR{
				{OriginalPR: 1234, CherryPickPR: 5678, Status: cmd.BranchStatusPicked},
			},
		},
		{
			name: "single merged PR",
			config: &cmd.Config{
				TrackedPRs: []cmd.TrackedPR{
					{
						Number: 1234,
						Branches: map[string]cmd.BranchStatus{
							"release-3.7": {
								Status: cmd.BranchStatusMerged,
								PR: &cmd.PickPR{
									Number: 5678,
								},
							},
						},
					},
				},
			},
			targetBranch: "release-3.7",
			expected: []PickedPR{
				{OriginalPR: 1234, CherryPickPR: 5678, Status: cmd.BranchStatusMerged},
			},
		},
		{
			name: "multiple PRs",
			config: &cmd.Config{
				TrackedPRs: []cmd.TrackedPR{
					{
						Number: 1234,
						Branches: map[string]cmd.BranchStatus{
							"release-3.7": {
								Status: cmd.BranchStatusPicked,
								PR: &cmd.PickPR{
									Number: 5678,
								},
							},
						},
					},
					{
						Number: 2345,
						Branches: map[string]cmd.BranchStatus{
							"release-3.7": {
								Status: cmd.BranchStatusMerged,
								PR: &cmd.PickPR{
									Number: 6789,
								},
							},
						},
					},
				},
			},
			targetBranch: "release-3.7",
			expected: []PickedPR{
				{OriginalPR: 1234, CherryPickPR: 5678, Status: cmd.BranchStatusPicked},
				{OriginalPR: 2345, CherryPickPR: 6789, Status: cmd.BranchStatusMerged},
			},
		},
		{
			name: "pending status not included",
			config: &cmd.Config{
				TrackedPRs: []cmd.TrackedPR{
					{
						Number: 1234,
						Branches: map[string]cmd.BranchStatus{
							"release-3.7": {
								Status: cmd.BranchStatusPending,
							},
						},
					},
				},
			},
			targetBranch: "release-3.7",
			expected:     nil,
		},
		{
			name: "failed status not included",
			config: &cmd.Config{
				TrackedPRs: []cmd.TrackedPR{
					{
						Number: 1234,
						Branches: map[string]cmd.BranchStatus{
							"release-3.7": {
								Status: cmd.BranchStatusFailed,
							},
						},
					},
				},
			},
			targetBranch: "release-3.7",
			expected:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPickedPRs(tt.config, tt.targetBranch)

			if len(got) != len(tt.expected) {
				t.Errorf("getPickedPRs() returned %d PRs, want %d", len(got), len(tt.expected))
				return
			}

			for i, expected := range tt.expected {
				if got[i].OriginalPR != expected.OriginalPR ||
					got[i].CherryPickPR != expected.CherryPickPR ||
					got[i].Status != expected.Status {
					t.Errorf("getPickedPRs()[%d] = %v, want %v", i, got[i], expected)
				}
			}
		})
	}
}
