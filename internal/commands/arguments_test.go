package commands

import (
	"testing"

	"github.com/alan/cherry-picker/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantPRNumber, result.PRNumber)
				assert.Equal(t, tt.wantBranch, result.TargetBranch)
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
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
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
			got := GetTargetBranchFromArgs(tt.args)
			assert.Equal(t, tt.want, got)
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
			assert.Len(t, got, tt.wantLen)
			if tt.wantContains != "" {
				assert.Contains(t, got, tt.wantContains)
			}
		})
	}
}
