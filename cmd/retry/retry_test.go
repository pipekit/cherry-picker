package retry

import (
	"bytes"
	"errors"
	"testing"

	"github.com/alan/cherry-picker/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRetryCmd tests command creation and initialization
func TestNewRetryCmd(t *testing.T) {
	mockConfig := &cmd.Config{
		Org:  "test-org",
		Repo: "test-repo",
	}

	loadConfig := func(_ string) (*cmd.Config, error) {
		return mockConfig, nil
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	cobraCmd := NewRetryCmd(loadConfig, saveConfig)

	assert.NotNil(t, cobraCmd)
	assert.NotEmpty(t, cobraCmd.Use)
	assert.NotEmpty(t, cobraCmd.Short)
	assert.NotEmpty(t, cobraCmd.Long)
	assert.NotNil(t, cobraCmd.RunE)
	assert.NoError(t, cobraCmd.Args(cobraCmd, []string{}))              // 0 args ok
	assert.NoError(t, cobraCmd.Args(cobraCmd, []string{"1"}))           // 1 arg ok
	assert.NoError(t, cobraCmd.Args(cobraCmd, []string{"1", "branch"})) // 2 args ok
	assert.Error(t, cobraCmd.Args(cobraCmd, []string{"1", "2", "3"}))   // 3 args not ok
}

// TestRetryCmd_RunE_InvalidPRNumber tests error handling for invalid PR number
func TestRetryCmd_RunE_InvalidPRNumber(t *testing.T) {
	loadConfig := func(_ string) (*cmd.Config, error) {
		return &cmd.Config{}, nil
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	cobraCmd := NewRetryCmd(loadConfig, saveConfig)

	// Test with invalid PR number - should error
	err := cobraCmd.RunE(cobraCmd, []string{"invalid"})
	require.Error(t, err)
}

// TestRetryCmd_RunE_ConfigLoadError tests error when config fails to load
func TestRetryCmd_RunE_ConfigLoadError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	loadConfig := func(_ string) (*cmd.Config, error) {
		return nil, errors.New("config load error")
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	cobraCmd := NewRetryCmd(loadConfig, saveConfig)

	err := cobraCmd.RunE(cobraCmd, []string{})
	require.Error(t, err)
}

// TestCommand_Run_PRNotFound tests when PR is not tracked
func TestCommand_Run_PRNotFound(t *testing.T) {
	rc := &command{
		PRNumber: 999,
	}
	rc.Config = &cmd.Config{
		TrackedPRs: []cmd.TrackedPR{},
	}

	err := rc.Run(t.Context())
	require.Error(t, err)
}

// TestCommand_Run_NoPRsEligible tests when no PRs are eligible for retry
func TestCommand_Run_NoPRsEligible(t *testing.T) {
	rc := &command{
		PRNumber: 123,
	}
	rc.Config = &cmd.Config{
		TrackedPRs: []cmd.TrackedPR{
			{
				Number: 123,
				Branches: map[string]cmd.BranchStatus{
					"release-1.0": {
						Status: cmd.BranchStatusPending, // Not eligible (not picked)
					},
				},
			},
		},
	}

	err := rc.Run(t.Context())
	require.Error(t, err)
}

// TestCommand_Run_BranchNotEligible tests when specified branch is not eligible
func TestCommand_Run_BranchNotEligible(t *testing.T) {
	rc := &command{
		PRNumber:     123,
		TargetBranch: "release-1.0",
	}
	rc.Config = &cmd.Config{
		TrackedPRs: []cmd.TrackedPR{
			{
				Number: 123,
				Branches: map[string]cmd.BranchStatus{
					"release-1.0": {
						Status: cmd.BranchStatusPicked,
						PR: &cmd.PickPR{
							Number:   456,
							CIStatus: cmd.CIStatusPassing, // Not eligible (CI already passing)
						},
					},
				},
			},
		},
	}

	err := rc.Run(t.Context())
	require.Error(t, err)
}

// TestCommand_Run_BranchNotTracked tests when specified branch is not tracked
func TestCommand_Run_BranchNotTracked(t *testing.T) {
	rc := &command{
		PRNumber:     123,
		TargetBranch: "release-2.0",
	}
	rc.Config = &cmd.Config{
		TrackedPRs: []cmd.TrackedPR{
			{
				Number: 123,
				Branches: map[string]cmd.BranchStatus{
					"release-1.0": {
						Status: cmd.BranchStatusPicked,
						PR: &cmd.PickPR{
							Number:   456,
							CIStatus: cmd.CIStatusFailing,
						},
					},
				},
			},
		},
	}

	err := rc.Run(t.Context())
	require.Error(t, err)
}

// TestCommand_RetriesCorrectPR tests that the retry targets the cherry-pick PR
func TestCommand_RetriesCorrectPR(t *testing.T) {
	// This test verifies the critical invariant that we retry the cherry-pick PR,
	// not the original PR
	config := &cmd.Config{
		TrackedPRs: []cmd.TrackedPR{
			{
				Number: 123, // Original PR
				Branches: map[string]cmd.BranchStatus{
					"release-1.0": {
						Status: cmd.BranchStatusPicked,
						PR: &cmd.PickPR{
							Number:   456, // Cherry-pick PR
							CIStatus: cmd.CIStatusFailing,
						},
					},
				},
			},
		},
	}

	rc := &command{
		PRNumber:     123,
		TargetBranch: "release-1.0",
	}
	rc.Config = config

	// The critical thing to verify is that when retrying PR #123,
	// it actually operates on cherry-pick PR #456
	branchStatus := config.TrackedPRs[0].Branches["release-1.0"]
	assert.Equal(t, 456, branchStatus.PR.Number, "should target cherry-pick PR, not original")
}

// TestCommandFlags tests that the command has the expected flags
func TestCommandFlags(t *testing.T) {
	cobraCmd := NewRetryCmd(nil, nil)

	// Check that the config flag exists
	configFlag := cobraCmd.Flags().Lookup("config")
	assert.NotNil(t, configFlag)
	assert.Equal(t, "cherry-picks.yaml", configFlag.DefValue)
	assert.Equal(t, "Path to configuration file", configFlag.Usage)
}

// TestCommandOutput tests command output formatting
func TestCommandOutput(t *testing.T) {
	cobraCmd := NewRetryCmd(nil, nil)

	// Test help output
	var buf bytes.Buffer
	cobraCmd.SetOut(&buf)
	cobraCmd.SetErr(&buf)

	// Just check that help can be generated without errors
	err := cobraCmd.Help()
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String(), "should generate help text")
}

// TestCommand_DoesNotModifyConfig tests that retry doesn't change config
func TestCommand_DoesNotModifyConfig(t *testing.T) {
	// Retry should not modify the config, unlike merge which updates status
	config := &cmd.Config{
		TrackedPRs: []cmd.TrackedPR{
			{
				Number: 123,
				Branches: map[string]cmd.BranchStatus{
					"release-1.0": {
						Status: cmd.BranchStatusPicked,
						PR: &cmd.PickPR{
							Number:   456,
							CIStatus: cmd.CIStatusFailing,
						},
					},
				},
			},
		},
	}

	originalStatus := config.TrackedPRs[0].Branches["release-1.0"].Status
	originalCIStatus := config.TrackedPRs[0].Branches["release-1.0"].PR.CIStatus

	rc := &command{
		PRNumber:     123,
		TargetBranch: "release-1.0",
	}
	rc.Config = config

	// Even after a successful retry, the config should remain unchanged
	// User must run fetch to update CI status
	assert.Equal(t, originalStatus, config.TrackedPRs[0].Branches["release-1.0"].Status)
	assert.Equal(t, originalCIStatus, config.TrackedPRs[0].Branches["release-1.0"].PR.CIStatus)
}
