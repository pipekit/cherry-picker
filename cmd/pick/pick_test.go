package pick

import (
	"bytes"
	"errors"
	"testing"

	"github.com/alan/cherry-picker/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewPickCmd tests command creation and initialization
func TestNewPickCmd(t *testing.T) {
	loadConfig := func(_ string) (*cmd.Config, error) {
		return &cmd.Config{}, nil
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	cobraCmd := NewPickCmd(&configFile, loadConfig, saveConfig)

	assert.NotNil(t, cobraCmd)
	assert.NotEmpty(t, cobraCmd.Use)
	assert.NotEmpty(t, cobraCmd.Short)
	assert.NotEmpty(t, cobraCmd.Long)
	assert.NotNil(t, cobraCmd.RunE)

	// Test argument validation
	require.Error(t, cobraCmd.Args(cobraCmd, []string{}))                  // Requires at least 1 arg
	require.NoError(t, cobraCmd.Args(cobraCmd, []string{"123"}))           // 1 arg ok
	require.NoError(t, cobraCmd.Args(cobraCmd, []string{"123", "branch"})) // 2 args ok
	require.Error(t, cobraCmd.Args(cobraCmd, []string{"1", "2", "3"}))     // 3 args not ok
}

// TestPickCmd_RunE_InvalidPRNumber tests error handling for invalid PR number
func TestPickCmd_RunE_InvalidPRNumber(t *testing.T) {
	loadConfig := func(_ string) (*cmd.Config, error) {
		return &cmd.Config{}, nil
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	cobraCmd := NewPickCmd(&configFile, loadConfig, saveConfig)

	err := cobraCmd.RunE(cobraCmd, []string{"invalid"})
	require.Error(t, err)
}

// TestPickCmd_RunE_ConfigLoadError tests error when config fails to load
func TestPickCmd_RunE_ConfigLoadError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	loadConfig := func(_ string) (*cmd.Config, error) {
		return nil, errors.New("config load error")
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	cobraCmd := NewPickCmd(&configFile, loadConfig, saveConfig)

	err := cobraCmd.RunE(cobraCmd, []string{"123"})
	require.Error(t, err)
}

// TestCommand_Run_PRNotTracked tests error when PR is not tracked
func TestCommand_Run_PRNotTracked(t *testing.T) {
	configFile := "test-config.yaml"
	loadConfig := func(_ string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:                "testorg",
			AIAssistantCommand: "cursor-agent",
			Repo:               "testrepo",
			SourceBranch:       "main",
			TrackedPRs:         []cmd.TrackedPR{},
		}, nil
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	pickCmd := &command{
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

// TestCommand_Run_BranchNotFailed tests that pick requires 'failed' status
func TestCommand_Run_BranchNotFailed(t *testing.T) {
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
			pickCmd := &command{
				PRNumber:     123,
				TargetBranch: "release-1.0",
			}
			pickCmd.ConfigFile = &configFile
			pickCmd.LoadConfig = func(_ string) (*cmd.Config, error) {
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
			pickCmd.SaveConfig = func(_ string, _ *cmd.Config) error {
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

// TestCommand_Run_BranchNotTracked tests error when branch is not in PR
func TestCommand_Run_BranchNotTracked(t *testing.T) {
	configFile := "test-config.yaml"
	pickCmd := &command{
		PRNumber:     123,
		TargetBranch: "release-2.0", // Branch not in PR
	}
	pickCmd.ConfigFile = &configFile
	pickCmd.LoadConfig = func(_ string) (*cmd.Config, error) {
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
	pickCmd.SaveConfig = func(_ string, _ *cmd.Config) error {
		return nil
	}

	config, err := pickCmd.LoadConfig(configFile)
	require.NoError(t, err)
	pickCmd.Config = config

	err = pickCmd.runPickForTest()
	require.Error(t, err)
}

// TestCommand_Run_SuccessfulPick tests successful cherry-pick operation
func TestCommand_Run_SuccessfulPick(t *testing.T) {
	configFile := "test-config.yaml"
	var savedConfig *cmd.Config

	loadConfig := func(_ string) (*cmd.Config, error) {
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
	saveConfig := func(_ string, config *cmd.Config) error {
		savedConfig = config
		return nil
	}

	pickCmd := &command{
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

// TestCommand_Run_MultipleFailedBranches tests picking multiple failed branches
func TestCommand_Run_MultipleFailedBranches(t *testing.T) {
	configFile := "test-config.yaml"
	var savedConfig *cmd.Config

	loadConfig := func(_ string) (*cmd.Config, error) {
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
	saveConfig := func(_ string, config *cmd.Config) error {
		savedConfig = config
		return nil
	}

	pickCmd := &command{
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

// TestCommand_Run_ConfigSaveError tests error handling when config save fails
func TestCommand_Run_ConfigSaveError(t *testing.T) {
	configFile := "test-config.yaml"

	loadConfig := func(_ string) (*cmd.Config, error) {
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
	saveConfig := func(_ string, _ *cmd.Config) error {
		return errors.New("permission denied")
	}

	pickCmd := &command{
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
			pc := &command{PRNumber: tt.pr.Number}
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
	pc := &command{}
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
	pc := &command{}
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

// TestCommandOutput tests command output formatting
func TestCommandOutput(t *testing.T) {
	configFile := "test-config.yaml"
	cobraCmd := NewPickCmd(&configFile, nil, nil)

	// Test help output
	var buf bytes.Buffer
	cobraCmd.SetOut(&buf)
	cobraCmd.SetErr(&buf)

	err := cobraCmd.Help()
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String(), "should generate help text")
}
