package commands

import (
	"testing"

	"github.com/alan/cherry-picker/cmd"
)

func TestFindAndValidatePR(t *testing.T) {
	config := &cmd.Config{
		TrackedPRs: []cmd.TrackedPR{
			{Number: 123, Title: "Test PR 1"},
			{Number: 456, Title: "Test PR 2"},
		},
	}

	tests := []struct {
		name     string
		prNumber int
		wantErr  bool
		expected *cmd.TrackedPR
	}{
		{
			name:     "found PR",
			prNumber: 123,
			wantErr:  false,
			expected: &config.TrackedPRs[0],
		},
		{
			name:     "PR not found",
			prNumber: 999,
			wantErr:  true,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FindAndValidatePR(config, tt.prNumber)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindAndValidatePR() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result.Number != tt.expected.Number {
				t.Errorf("FindAndValidatePR() = %v, want %v", result.Number, tt.expected.Number)
			}
		})
	}
}

func TestValidateTargetBranch(t *testing.T) {
	pr := &cmd.TrackedPR{
		Number: 123,
		Branches: map[string]cmd.BranchStatus{
			"release-3.6": {Status: cmd.BranchStatusPending},
			"release-3.7": {Status: cmd.BranchStatusPicked},
		},
	}

	tests := []struct {
		name         string
		targetBranch string
		wantErr      bool
	}{
		{
			name:         "empty branch (all branches)",
			targetBranch: "",
			wantErr:      false,
		},
		{
			name:         "existing branch",
			targetBranch: "release-3.6",
			wantErr:      false,
		},
		{
			name:         "non-existing branch",
			targetBranch: "release-3.8",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTargetBranch(pr, tt.targetBranch)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTargetBranch() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateBranchForOperation(t *testing.T) {
	pr := &cmd.TrackedPR{
		Number: 123,
		Branches: map[string]cmd.BranchStatus{
			"release-3.6": {
				Status: cmd.BranchStatusPicked,
				PR: &cmd.PickPR{
					Number:   789,
					CIStatus: cmd.CIStatusPassing,
				},
			},
			"release-3.7": {
				Status: cmd.BranchStatusPending,
			},
			"release-3.8": {
				Status: cmd.BranchStatusPicked,
				PR: &cmd.PickPR{
					Number:   790,
					CIStatus: cmd.CIStatusFailing,
				},
			},
		},
	}

	tests := []struct {
		name         string
		targetBranch string
		operation    string
		predicate    BranchValidationPredicate
		wantErr      bool
	}{
		{
			name:         "valid branch for merge",
			targetBranch: "release-3.6",
			operation:    "merge",
			predicate:    IsEligibleForMerge,
			wantErr:      false,
		},
		{
			name:         "branch not tracked",
			targetBranch: "release-3.9",
			operation:    "merge",
			predicate:    IsEligibleForMerge,
			wantErr:      true,
		},
		{
			name:         "branch not picked",
			targetBranch: "release-3.7",
			operation:    "merge",
			predicate:    IsEligibleForMerge,
			wantErr:      true,
		},
		{
			name:         "branch failing predicate",
			targetBranch: "release-3.8",
			operation:    "merge",
			predicate:    IsEligibleForMerge,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBranchForOperation(pr, tt.targetBranch, tt.operation, tt.predicate)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBranchForOperation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAnyBranchForOperation(t *testing.T) {
	tests := []struct {
		name      string
		pr        *cmd.TrackedPR
		operation string
		predicate BranchValidationPredicate
		wantErr   bool
	}{
		{
			name: "has eligible branch for merge",
			pr: &cmd.TrackedPR{
				Number: 123,
				Branches: map[string]cmd.BranchStatus{
					"release-3.6": {
						Status: cmd.BranchStatusPicked,
						PR: &cmd.PickPR{
							Number:   789,
							CIStatus: cmd.CIStatusPassing,
						},
					},
				},
			},
			operation: "merge",
			predicate: IsEligibleForMerge,
			wantErr:   false,
		},
		{
			name: "no eligible branches",
			pr: &cmd.TrackedPR{
				Number: 123,
				Branches: map[string]cmd.BranchStatus{
					"release-3.6": {
						Status: cmd.BranchStatusPending,
					},
					"release-3.7": {
						Status: cmd.BranchStatusPicked,
						PR: &cmd.PickPR{
							Number:   789,
							CIStatus: cmd.CIStatusFailing,
						},
					},
				},
			},
			operation: "merge",
			predicate: IsEligibleForMerge,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAnyBranchForOperation(tt.pr, tt.operation, tt.predicate)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAnyBranchForOperation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsEligibleForMerge(t *testing.T) {
	tests := []struct {
		name   string
		status cmd.BranchStatus
		want   bool
	}{
		{
			name: "eligible for merge",
			status: cmd.BranchStatus{
				Status: cmd.BranchStatusPicked,
				PR: &cmd.PickPR{
					Number:   789,
					CIStatus: cmd.CIStatusPassing,
				},
			},
			want: true,
		},
		{
			name: "not picked",
			status: cmd.BranchStatus{
				Status: cmd.BranchStatusPending,
			},
			want: false,
		},
		{
			name: "no PR info",
			status: cmd.BranchStatus{
				Status: cmd.BranchStatusPicked,
				PR:     nil,
			},
			want: false,
		},
		{
			name: "CI not passing",
			status: cmd.BranchStatus{
				Status: cmd.BranchStatusPicked,
				PR: &cmd.PickPR{
					Number:   789,
					CIStatus: cmd.CIStatusFailing,
				},
			},
			want: false,
		},
		{
			name: "already merged",
			status: cmd.BranchStatus{
				Status: cmd.BranchStatusMerged,
				PR: &cmd.PickPR{
					Number:   789,
					CIStatus: cmd.CIStatusPassing,
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEligibleForMerge(tt.status); got != tt.want {
				t.Errorf("IsEligibleForMerge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsEligibleForRetry(t *testing.T) {
	tests := []struct {
		name   string
		status cmd.BranchStatus
		want   bool
	}{
		{
			name: "eligible for retry",
			status: cmd.BranchStatus{
				Status: cmd.BranchStatusPicked,
				PR: &cmd.PickPR{
					Number:   789,
					CIStatus: cmd.CIStatusFailing,
				},
			},
			want: true,
		},
		{
			name: "not picked",
			status: cmd.BranchStatus{
				Status: cmd.BranchStatusPending,
			},
			want: false,
		},
		{
			name: "no PR info",
			status: cmd.BranchStatus{
				Status: cmd.BranchStatusPicked,
				PR:     nil,
			},
			want: false,
		},
		{
			name: "CI passing (no retry needed)",
			status: cmd.BranchStatus{
				Status: cmd.BranchStatusPicked,
				PR: &cmd.PickPR{
					Number:   789,
					CIStatus: cmd.CIStatusPassing,
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEligibleForRetry(tt.status); got != tt.want {
				t.Errorf("IsEligibleForRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}
