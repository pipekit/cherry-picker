package pick

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/alan/cherry-picker/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewPickCmd tests command creation and initialization
func TestNewPickCmd(t *testing.T) {
	loadConfig := func(filename string) (*cmd.Config, error) {
		return &cmd.Config{}, nil
	}
	saveConfig := func(filename string, config *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	cmd := NewPickCmd(&configFile, loadConfig, saveConfig)

	assert.NotNil(t, cmd)
	assert.NotEmpty(t, cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotNil(t, cmd.RunE)

	// Test argument validation
	assert.Error(t, cmd.Args(cmd, []string{})) // Requires at least 1 arg
	assert.NoError(t, cmd.Args(cmd, []string{"123"})) // 1 arg ok
	assert.NoError(t, cmd.Args(cmd, []string{"123", "branch"})) // 2 args ok
	assert.Error(t, cmd.Args(cmd, []string{"1", "2", "3"})) // 3 args not ok
}

// TestPickCmd_RunE_InvalidPRNumber tests error handling for invalid PR number
func TestPickCmd_RunE_InvalidPRNumber(t *testing.T) {
	loadConfig := func(path string) (*cmd.Config, error) {
		return &cmd.Config{}, nil
	}
	saveConfig := func(path string, config *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	cmd := NewPickCmd(&configFile, loadConfig, saveConfig)

	err := cmd.RunE(cmd, []string{"invalid"})
	require.Error(t, err)
}

// TestPickCmd_RunE_ConfigLoadError tests error when config fails to load
func TestPickCmd_RunE_ConfigLoadError(t *testing.T) {
	loadConfig := func(path string) (*cmd.Config, error) {
		return nil, errors.New("config load error")
	}
	saveConfig := func(path string, config *cmd.Config) error {
		return nil
	}

	// Set GITHUB_TOKEN to avoid that error
	oldToken := os.Getenv("GITHUB_TOKEN")
	os.Setenv("GITHUB_TOKEN", "test-token")
	defer os.Setenv("GITHUB_TOKEN", oldToken)

	configFile := "test-config.yaml"
	cmd := NewPickCmd(&configFile, loadConfig, saveConfig)

	err := cmd.RunE(cmd, []string{"123"})
	require.Error(t, err)
}

// TestPickCommand_Run_PRNotTracked tests error when PR is not tracked
func TestPickCommand_Run_PRNotTracked(t *testing.T) {
	configFile := "test-config.yaml"
	loadConfig := func(filename string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:                "testorg",
			AIAssistantCommand: "cursor-agent",
			Repo:               "testrepo",
			SourceBranch:       "main",
			TrackedPRs:         []cmd.TrackedPR{},
		}, nil
	}
	saveConfig := func(filename string, config *cmd.Config) error {
		return nil
	}

	pickCmd := &PickCommand{
		PRNumber:     123,
		TargetBranch: "release-1.0",
	}
	pickCmd.ConfigFile = &configFile
	pickCmd.LoadConfig = loadConfig
	pickCmd.SaveConfig = saveConfig

	config, err := loadConfig(configFile)
	require.NoError(t, err)
	pickCmd.Config = config

	err = pickCmd.runPickForTest()
	require.Error(t, err)
}

// TestPickCommand_Run_BranchNotFailed tests that pick requires 'failed' status
func TestPickCommand_Run_BranchNotFailed(t *testing.T) {
	tests := []struct {
		name   string
		status cmd.BranchStatusType
	}{
		{"pending status", cmd.BranchStatusPending},
		{"picked status", cmd.BranchStatusPicked},
		{"merged status", cmd.BranchStatusMerged},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := "test-config.yaml"
			pickCmd := &PickCommand{
				PRNumber:     123,
				TargetBranch: "release-1.0",
			}
			pickCmd.ConfigFile = &configFile
			pickCmd.LoadConfig = func(filename string) (*cmd.Config, error) {
				return &cmd.Config{
					Org:                "testorg",
					AIAssistantCommand: "cursor-agent",
					Repo:               "testrepo",
					SourceBranch:       "main",
					TrackedPRs: []cmd.TrackedPR{
						{
							Number: 123,
							Title:  "Test PR",
							Branches: map[string]cmd.BranchStatus{
								"release-1.0": {Status: tt.status},
							},
						},
					},
				}, nil
			}
			pickCmd.SaveConfig = func(filename string, config *cmd.Config) error {
				return nil
			}

			config, err := pickCmd.LoadConfig(configFile)
			require.NoError(t, err)
			pickCmd.Config = config

			err = pickCmd.runPickForTest()
			require.Error(t, err)
		})
	}
}

// TestPickCommand_Run_BranchNotTracked tests error when branch is not in PR
func TestPickCommand_Run_BranchNotTracked(t *testing.T) {
	configFile := "test-config.yaml"
	pickCmd := &PickCommand{
		PRNumber:     123,
		TargetBranch: "release-2.0", // Branch not in PR
	}
	pickCmd.ConfigFile = &configFile
	pickCmd.LoadConfig = func(filename string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:                "testorg",
			AIAssistantCommand: "cursor-agent",
			Repo:               "testrepo",
			SourceBranch:       "main",
			TrackedPRs: []cmd.TrackedPR{
				{
					Number: 123,
					Title:  "Test PR",
					Branches: map[string]cmd.BranchStatus{
						"release-1.0": {Status: cmd.BranchStatusFailed},
					},
				},
			},
		}, nil
	}
	pickCmd.SaveConfig = func(filename string, config *cmd.Config) error {
		return nil
	}

	config, err := pickCmd.LoadConfig(configFile)
	require.NoError(t, err)
	pickCmd.Config = config

	err = pickCmd.runPickForTest()
	require.Error(t, err)
}

// TestPickCommand_Run_SuccessfulPick tests successful cherry-pick operation
func TestPickCommand_Run_SuccessfulPick(t *testing.T) {
	configFile := "test-config.yaml"
	var savedConfig *cmd.Config

	loadConfig := func(filename string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:                "testorg",
			AIAssistantCommand: "cursor-agent",
			Repo:               "testrepo",
			SourceBranch:       "main",
			TrackedPRs: []cmd.TrackedPR{
				{
					Number: 123,
					Title:  "Test PR",
					Branches: map[string]cmd.BranchStatus{
						"release-1.0": {Status: cmd.BranchStatusFailed},
					},
				},
			},
		}, nil
	}
	saveConfig := func(filename string, config *cmd.Config) error {
		savedConfig = config
		return nil
	}

	pickCmd := &PickCommand{
		PRNumber:     123,
		TargetBranch: "release-1.0",
	}
	pickCmd.ConfigFile = &configFile
	pickCmd.LoadConfig = loadConfig
	pickCmd.SaveConfig = saveConfig

	config, err := loadConfig(configFile)
	require.NoError(t, err)
	pickCmd.Config = config

	err = pickCmd.runPickForTest()
	require.NoError(t, err)

	// Verify config was saved
	require.NotNil(t, savedConfig)

	// Verify the PR status was updated to picked
	pr := &savedConfig.TrackedPRs[0]
	assert.Equal(t, cmd.BranchStatusPicked, pr.Branches["release-1.0"].Status)
}

// TestPickCommand_Run_MultipleFailedBranches tests picking multiple failed branches
func TestPickCommand_Run_MultipleFailedBranches(t *testing.T) {
	configFile := "test-config.yaml"
	var savedConfig *cmd.Config

	loadConfig := func(filename string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:                "testorg",
			AIAssistantCommand: "cursor-agent",
			Repo:               "testrepo",
			SourceBranch:       "main",
			TrackedPRs: []cmd.TrackedPR{
				{
					Number: 123,
					Title:  "Test PR",
					Branches: map[string]cmd.BranchStatus{
						"release-1.0": {Status: cmd.BranchStatusFailed},
						"release-2.0": {Status: cmd.BranchStatusFailed},
					},
				},
			},
		}, nil
	}
	saveConfig := func(filename string, config *cmd.Config) error {
		savedConfig = config
		return nil
	}

	pickCmd := &PickCommand{
		PRNumber:     123,
		TargetBranch: "", // No specific branch = all branches
	}
	pickCmd.ConfigFile = &configFile
	pickCmd.LoadConfig = loadConfig
	pickCmd.SaveConfig = saveConfig

	config, err := loadConfig(configFile)
	require.NoError(t, err)
	pickCmd.Config = config

	err = pickCmd.runPickForTest()
	require.NoError(t, err)

	// Verify both failed branches were updated to picked
	pr := &savedConfig.TrackedPRs[0]
	assert.Equal(t, cmd.BranchStatusPicked, pr.Branches["release-1.0"].Status)
	assert.Equal(t, cmd.BranchStatusPicked, pr.Branches["release-2.0"].Status)
}

// TestPickCommand_Run_ConfigSaveError tests error handling when config save fails
func TestPickCommand_Run_ConfigSaveError(t *testing.T) {
	configFile := "test-config.yaml"

	loadConfig := func(filename string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:                "testorg",
			AIAssistantCommand: "cursor-agent",
			Repo:               "testrepo",
			SourceBranch:       "main",
			TrackedPRs: []cmd.TrackedPR{
				{
					Number: 123,
					Title:  "Test PR",
					Branches: map[string]cmd.BranchStatus{
						"release-1.0": {Status: cmd.BranchStatusFailed},
					},
				},
			},
		}, nil
	}
	saveConfig := func(filename string, config *cmd.Config) error {
		return errors.New("permission denied")
	}

	pickCmd := &PickCommand{
		PRNumber:     123,
		TargetBranch: "release-1.0",
	}
	pickCmd.ConfigFile = &configFile
	pickCmd.LoadConfig = loadConfig
	pickCmd.SaveConfig = saveConfig

	config, err := loadConfig(configFile)
	require.NoError(t, err)
	pickCmd.Config = config

	err = pickCmd.runPickForTest()
	require.Error(t, err)
}

// TestValidatePickableStatus tests the validation logic for pickable branches
func TestValidatePickableStatus(t *testing.T) {
	tests := []struct {
		name     string
		pr       *cmd.TrackedPR
		branches []string
		wantErr  bool
	}{
		{
			name: "all branches failed - valid",
			pr: &cmd.TrackedPR{
				Number: 123,
				Branches: map[string]cmd.BranchStatus{
					"release-1.0": {Status: cmd.BranchStatusFailed},
					"release-2.0": {Status: cmd.BranchStatusFailed},
				},
			},
			branches: []string{"release-1.0", "release-2.0"},
			wantErr:  false,
		},
		{
			name: "one branch not failed - invalid",
			pr: &cmd.TrackedPR{
				Number: 123,
				Branches: map[string]cmd.BranchStatus{
					"release-1.0": {Status: cmd.BranchStatusFailed},
					"release-2.0": {Status: cmd.BranchStatusPending},
				},
			},
			branches: []string{"release-1.0", "release-2.0"},
			wantErr:  true,
		},
		{
			name: "branch not exists - invalid",
			pr: &cmd.TrackedPR{
				Number: 123,
				Branches: map[string]cmd.BranchStatus{
					"release-1.0": {Status: cmd.BranchStatusFailed},
				},
			},
			branches: []string{"release-1.0", "release-3.0"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &PickCommand{PRNumber: tt.pr.Number}
			err := pc.validatePickableStatus(tt.pr, tt.branches)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestUpdatePRStatus tests the status update logic
func TestUpdatePRStatus(t *testing.T) {
	pc := &PickCommand{}
	pr := &cmd.TrackedPR{
		Number: 123,
		Branches: map[string]cmd.BranchStatus{
			"release-1.0": {Status: cmd.BranchStatusFailed},
			"release-2.0": {Status: cmd.BranchStatusFailed},
		},
	}

	branches := []string{"release-1.0"}
	pc.updatePRStatus(pr, branches)

	assert.Equal(t, cmd.BranchStatusPicked, pr.Branches["release-1.0"].Status)
	assert.Nil(t, pr.Branches["release-1.0"].PR, "PR details should be nil initially")
	assert.Equal(t, cmd.BranchStatusFailed, pr.Branches["release-2.0"].Status, "unchanged branch")
}

// TestUpdateSingleBranchStatus tests updating a single branch with result
func TestUpdateSingleBranchStatus(t *testing.T) {
	pc := &PickCommand{}
	pr := &cmd.TrackedPR{
		Number: 123,
		Branches: map[string]cmd.BranchStatus{
			"release-1.0": {Status: cmd.BranchStatusFailed},
		},
	}

	result := &CherryPickResult{
		PRNumber: 456,
		Title:    "Cherry-pick PR",
		CIStatus: "pending",
	}

	pc.updateSingleBranchStatus(pr, "release-1.0", result)

	assert.Equal(t, cmd.BranchStatusPicked, pr.Branches["release-1.0"].Status)
	assert.NotNil(t, pr.Branches["release-1.0"].PR)
	assert.Equal(t, 456, pr.Branches["release-1.0"].PR.Number)
	assert.Equal(t, "Cherry-pick PR", pr.Branches["release-1.0"].PR.Title)
	assert.Equal(t, cmd.CIStatusPending, pr.Branches["release-1.0"].PR.CIStatus)
}

// TestPickCommandOutput tests command output formatting
func TestPickCommandOutput(t *testing.T) {
	configFile := "test-config.yaml"
	cmd := NewPickCmd(&configFile, nil, nil)

	// Test help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Help()
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String(), "should generate help text")
}