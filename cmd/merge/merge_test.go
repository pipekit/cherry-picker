package merge

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/alan/cherry-picker/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMergeCmd tests command creation and initialization
func TestNewMergeCmd(t *testing.T) {
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

	cmd := NewMergeCmd(loadConfig, saveConfig)

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

// TestMergeCmd_RunE_InvalidPRNumber tests error handling for invalid PR number
func TestMergeCmd_RunE_InvalidPRNumber(t *testing.T) {
	loadConfig := func(path string) (*cmd.Config, error) {
		return &cmd.Config{}, nil
	}
	saveConfig := func(path string, config *cmd.Config) error {
		return nil
	}

	cmd := NewMergeCmd(loadConfig, saveConfig)

	// Test with invalid PR number - should error
	err := cmd.RunE(cmd, []string{"invalid"})
	require.Error(t, err)
}

// TestMergeCmd_RunE_ConfigLoadError tests error when config fails to load
func TestMergeCmd_RunE_ConfigLoadError(t *testing.T) {
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

	cmd := NewMergeCmd(loadConfig, saveConfig)

	err := cmd.RunE(cmd, []string{})
	require.Error(t, err)
}

// TestMergeCommand_Run_PRNotFound tests when PR is not tracked
func TestMergeCommand_Run_PRNotFound(t *testing.T) {
	mc := &MergeCommand{
		PRNumber: 999,
	}
	mc.Config = &cmd.Config{
		TrackedPRs: []cmd.TrackedPR{},
	}

	err := mc.Run()
	require.Error(t, err)
}

// TestMergeCommand_Run_NoPRsEligible tests when no PRs are eligible for merge
func TestMergeCommand_Run_NoPRsEligible(t *testing.T) {
	mc := &MergeCommand{
		PRNumber: 123,
	}
	mc.Config = &cmd.Config{
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

	err := mc.Run()
	require.Error(t, err)
}

// TestMergeCommand_Run_BranchNotEligible tests when specified branch is not eligible
func TestMergeCommand_Run_BranchNotEligible(t *testing.T) {
	mc := &MergeCommand{
		PRNumber:     123,
		TargetBranch: "release-1.0",
	}
	mc.Config = &cmd.Config{
		TrackedPRs: []cmd.TrackedPR{
			{
				Number: 123,
				Branches: map[string]cmd.BranchStatus{
					"release-1.0": {
						Status: cmd.BranchStatusPicked,
						PR: &cmd.PickPR{
							Number:   456,
							CIStatus: cmd.CIStatusFailing, // Not eligible (CI failing)
						},
					},
				},
			},
		},
	}

	err := mc.Run()
	require.Error(t, err)
}

// TestMergeCommand_Run_BranchNotTracked tests when specified branch is not tracked
func TestMergeCommand_Run_BranchNotTracked(t *testing.T) {
	mc := &MergeCommand{
		PRNumber:     123,
		TargetBranch: "release-2.0",
	}
	mc.Config = &cmd.Config{
		TrackedPRs: []cmd.TrackedPR{
			{
				Number: 123,
				Branches: map[string]cmd.BranchStatus{
					"release-1.0": {
						Status: cmd.BranchStatusPicked,
						PR: &cmd.PickPR{
							Number:   456,
							CIStatus: cmd.CIStatusPassing,
						},
					},
				},
			},
		},
	}

	err := mc.Run()
	require.Error(t, err)
}

// TestMergeCommandFlags tests that the command has the expected flags
func TestMergeCommandFlags(t *testing.T) {
	cmd := NewMergeCmd(nil, nil)

	// Check that the config flag exists
	configFlag := cmd.Flags().Lookup("config")
	assert.NotNil(t, configFlag)
	assert.Equal(t, "cherry-picks.yaml", configFlag.DefValue)
	assert.Equal(t, "Path to configuration file", configFlag.Usage)
}

// TestMergeCommandOutput tests command output formatting
func TestMergeCommandOutput(t *testing.T) {
	cmd := NewMergeCmd(nil, nil)

	// Test help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Just check that help can be generated without errors
	err := cmd.Help()
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String(), "should generate help text")
}