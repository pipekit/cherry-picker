package commands

import (
	"testing"

	"github.com/alan/cherry-picker/cmd"
)

func TestParsePRCommandArgs(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantPRNumber int
		wantBranch   string
		wantErr      bool
	}{
		{
			name:         "no arguments",
			args:         []string{},
			wantPRNumber: 0,
			wantBranch:   "",
			wantErr:      false,
		},
		{
			name:         "PR number only",
			args:         []string{"123"},
			wantPRNumber: 123,
			wantBranch:   "",
			wantErr:      false,
		},
		{
			name:         "PR number and branch",
			args:         []string{"123", "release-3.6"},
			wantPRNumber: 123,
			wantBranch:   "release-3.6",
			wantErr:      false,
		},
		{
			name:         "invalid PR number",
			args:         []string{"abc"},
			wantPRNumber: 0,
			wantBranch:   "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParsePRCommandArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePRCommandArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if result.PRNumber != tt.wantPRNumber {
					t.Errorf("ParsePRCommandArgs() PRNumber = %v, want %v", result.PRNumber, tt.wantPRNumber)
				}
				if result.TargetBranch != tt.wantBranch {
					t.Errorf("ParsePRCommandArgs() TargetBranch = %v, want %v", result.TargetBranch, tt.wantBranch)
				}
			}
		})
	}
}

func TestParsePRNumberFromArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		required bool
		want     int
		wantErr  bool
	}{
		{
			name:     "no args, not required",
			args:     []string{},
			required: false,
			want:     0,
			wantErr:  false,
		},
		{
			name:     "no args, required",
			args:     []string{},
			required: true,
			want:     0,
			wantErr:  true,
		},
		{
			name:     "valid PR number",
			args:     []string{"123"},
			required: false,
			want:     123,
			wantErr:  false,
		},
		{
			name:     "invalid PR number",
			args:     []string{"abc"},
			required: false,
			want:     0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePRNumberFromArgs(tt.args, tt.required)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePRNumberFromArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParsePRNumberFromArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTargetBranchFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "no branch argument",
			args: []string{"123"},
			want: "",
		},
		{
			name: "has branch argument",
			args: []string{"123", "release-3.6"},
			want: "release-3.6",
		},
		{
			name: "empty args",
			args: []string{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetTargetBranchFromArgs(tt.args); got != tt.want {
				t.Errorf("GetTargetBranchFromArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetermineBranchesToUpdate(t *testing.T) {
	pr := &cmd.TrackedPR{
		Number: 123,
		Branches: map[string]cmd.BranchStatus{
			"release-3.6": {Status: cmd.BranchStatusPending},
			"release-3.7": {Status: cmd.BranchStatusPicked},
			"release-3.8": {Status: cmd.BranchStatusFailed},
		},
	}

	tests := []struct {
		name         string
		targetBranch string
		wantLen      int
		wantContains string
	}{
		{
			name:         "specific branch",
			targetBranch: "release-3.6",
			wantLen:      1,
			wantContains: "release-3.6",
		},
		{
			name:         "all branches",
			targetBranch: "",
			wantLen:      3,
			wantContains: "", // Don't check specific content for all branches
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineBranchesToUpdate(pr, tt.targetBranch)
			if len(got) != tt.wantLen {
				t.Errorf("DetermineBranchesToUpdate() length = %v, want %v", len(got), tt.wantLen)
			}
			if tt.wantContains != "" {
				found := false
				for _, branch := range got {
					if branch == tt.wantContains {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("DetermineBranchesToUpdate() does not contain %v", tt.wantContains)
				}
			}
		})
	}
}
