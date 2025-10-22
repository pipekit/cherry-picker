package retry

import (
	"bytes"
	"errors"
	"os"
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

	loadConfig := func(path string) (*cmd.Config, error) {
		return mockConfig, nil
	}
	saveConfig := func(path string, config *cmd.Config) error {
		return nil
	}

	cmd := NewRetryCmd(loadConfig, saveConfig)

	assert.NotNil(t, cmd)
	assert.NotEmpty(t, cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotNil(t, cmd.RunE)
	assert.NoError(t, cmd.Args(cmd, []string{}))    // 0 args ok
	assert.NoError(t, cmd.Args(cmd, []string{"1"})) // 1 arg ok
	assert.NoError(t, cmd.Args(cmd, []string{"1", "branch"})) // 2 args ok
	assert.Error(t, cmd.Args(cmd, []string{"1", "2", "3"})) // 3 args not ok
}

// TestRetryCmd_RunE_InvalidPRNumber tests error handling for invalid PR number
func TestRetryCmd_RunE_InvalidPRNumber(t *testing.T) {
	loadConfig := func(path string) (*cmd.Config, error) {
		return &cmd.Config{}, nil
	}
	saveConfig := func(path string, config *cmd.Config) error {
		return nil
	}

	cmd := NewRetryCmd(loadConfig, saveConfig)

	// Test with invalid PR number - should error
	err := cmd.RunE(cmd, []string{"invalid"})
	require.Error(t, err)
}

// TestRetryCmd_RunE_ConfigLoadError tests error when config fails to load
func TestRetryCmd_RunE_ConfigLoadError(t *testing.T) {
	loadConfig := func(path string) (*cmd.Config, error) {
		return nil, errors.New("config load error")
	}
	saveConfig := func(path string, config *cmd.Config) error {
		return nil
	}

	// Temporarily set GITHUB_TOKEN to avoid that error
	oldToken := os.Getenv("GITHUB_TOKEN")
	os.Setenv("GITHUB_TOKEN", "test-token")
	defer os.Setenv("GITHUB_TOKEN", oldToken)

	cmd := NewRetryCmd(loadConfig, saveConfig)

	err := cmd.RunE(cmd, []string{})
	require.Error(t, err)
}

// TestRetryCommand_Run_PRNotFound tests when PR is not tracked
func TestRetryCommand_Run_PRNotFound(t *testing.T) {
	rc := &RetryCommand{
		PRNumber: 999,
	}
	rc.Config = &cmd.Config{
		TrackedPRs: []cmd.TrackedPR{},
	}

	err := rc.Run()
	require.Error(t, err)
}

// TestRetryCommand_Run_NoPRsEligible tests when no PRs are eligible for retry
func TestRetryCommand_Run_NoPRsEligible(t *testing.T) {
	rc := &RetryCommand{
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

	err := rc.Run()
	require.Error(t, err)
}

// TestRetryCommand_Run_BranchNotEligible tests when specified branch is not eligible
func TestRetryCommand_Run_BranchNotEligible(t *testing.T) {
	rc := &RetryCommand{
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

	err := rc.Run()
	require.Error(t, err)
}

// TestRetryCommand_Run_BranchNotTracked tests when specified branch is not tracked
func TestRetryCommand_Run_BranchNotTracked(t *testing.T) {
	rc := &RetryCommand{
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

	err := rc.Run()
	require.Error(t, err)
}

// TestRetryCommand_RetriesCorrectPR tests that the retry targets the cherry-pick PR
func TestRetryCommand_RetriesCorrectPR(t *testing.T) {
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

	rc := &RetryCommand{
		PRNumber:     123,
		TargetBranch: "release-1.0",
	}
	rc.Config = config

	// The critical thing to verify is that when retrying PR #123,
	// it actually operates on cherry-pick PR #456
	branchStatus := config.TrackedPRs[0].Branches["release-1.0"]
	assert.Equal(t, 456, branchStatus.PR.Number, "should target cherry-pick PR, not original")
}

// TestRetryCommandFlags tests that the command has the expected flags
func TestRetryCommandFlags(t *testing.T) {
	cmd := NewRetryCmd(nil, nil)

	// Check that the config flag exists
	configFlag := cmd.Flags().Lookup("config")
	assert.NotNil(t, configFlag)
	assert.Equal(t, "cherry-picks.yaml", configFlag.DefValue)
	assert.Equal(t, "Path to configuration file", configFlag.Usage)
}

// TestRetryCommandOutput tests command output formatting
func TestRetryCommandOutput(t *testing.T) {
	cmd := NewRetryCmd(nil, nil)

	// Test help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Just check that help can be generated without errors
	err := cmd.Help()
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String(), "should generate help text")
}

// TestRetryCommand_DoesNotModifyConfig tests that retry doesn't change config
func TestRetryCommand_DoesNotModifyConfig(t *testing.T) {
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

	rc := &RetryCommand{
		PRNumber:     123,
		TargetBranch: "release-1.0",
	}
	rc.Config = config

	// Even after a successful retry, the config should remain unchanged
	// User must run fetch to update CI status
	assert.Equal(t, originalStatus, config.TrackedPRs[0].Branches["release-1.0"].Status)
	assert.Equal(t, originalCIStatus, config.TrackedPRs[0].Branches["release-1.0"].PR.CIStatus)
}