package pick

import (
	"strings"
	"testing"

	"github.com/alan/cherry-picker/cmd"
)

func TestNewPickCmd(t *testing.T) {
	// Mock functions
	loadConfig := func(filename string) (*cmd.Config, error) {
		return nil, nil
	}
	saveConfig := func(filename string, config *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	cmd := NewPickCmd(&configFile, loadConfig, saveConfig)

	// Test command properties
	if cmd.Use != "pick <pr-number> [target-branch]" {
		t.Errorf("NewPickCmd() Use = %v, want %v", cmd.Use, "pick <pr-number> [target-branch]")
	}

	if cmd.Short != "Mark a PR as picked for a specific target branch" {
		t.Errorf("NewPickCmd() Short = %v, want expected short description", cmd.Short)
	}
}

func TestRunPick_PRNotTracked(t *testing.T) {
	configFile := "test-config.yaml"
	loadConfig := func(filename string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:            "testorg",
			Repo:           "testrepo",
			SourceBranch:   "main",
			TargetBranches: []string{"release-1.0"},
			TrackedPRs:     []cmd.TrackedPR{}, // No PRs tracked
		}, nil
	}
	saveConfig := func(filename string, config *cmd.Config) error {
		return nil
	}

	// Create a PickCommand and test without git operations
	pickCmd := &PickCommand{
		PRNumber:     123,
		TargetBranch: "release-1.0",
	}
	pickCmd.ConfigFile = &configFile
	pickCmd.LoadConfig = loadConfig
	pickCmd.SaveConfig = saveConfig

	// Initialize without GitHub client (skip Init() to avoid GITHUB_TOKEN requirement)
	config, err := loadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	pickCmd.Config = config

	err = pickCmd.runWithGitOps(false) // Skip git operations for testing

	if err == nil {
		t.Error("runWithGitOps() expected error for non-tracked PR, got nil")
	}

	expectedError := "PR #123 not found in configuration"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("runWithGitOps() error = %v, want error containing %v", err, expectedError)
	}
}

func TestRunPick_PRIgnored(t *testing.T) {
	configFile := "test-config.yaml"
	loadConfig := func(filename string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:            "testorg",
			Repo:           "testrepo",
			SourceBranch:   "main",
			TargetBranches: []string{"release-1.0"},
			TrackedPRs: []cmd.TrackedPR{
				{Number: 123, Title: "Ignored PR", Ignored: true}, // PR is ignored
			},
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
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	pickCmd.Config = config

	err = pickCmd.runWithGitOps(false)

	if err == nil {
		t.Error("runWithGitOps() expected error for ignored PR, got nil")
	}

	expectedError := "PR #123 is marked as ignored"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("runWithGitOps() error = %v, want error containing %v", err, expectedError)
	}
}

func TestRunPick_SuccessfulPick(t *testing.T) {
	configFile := "test-config.yaml"
	var savedConfig *cmd.Config

	loadConfig := func(filename string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:            "testorg",
			Repo:           "testrepo",
			SourceBranch:   "main",
			TargetBranches: []string{"release-1.0"},
			TrackedPRs: []cmd.TrackedPR{
				{
					Number:  123,
					Title:   "Test PR",
					Ignored: false,
					Branches: map[string]cmd.BranchStatus{
						"release-1.0": {Status: cmd.BranchStatusPending},
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
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	pickCmd.Config = config

	err = pickCmd.runWithGitOps(false) // Skip git operations for testing

	if err != nil {
		t.Errorf("runWithGitOps() unexpected error: %v", err)
	}

	// Verify the PR status was updated
	if savedConfig == nil {
		t.Error("runWithGitOps() config was not saved")
		return
	}

	pr := &savedConfig.TrackedPRs[0]
	if pr.Branches["release-1.0"].Status != cmd.BranchStatusPicked {
		t.Errorf("runWithGitOps() branch status = %v, want %v", pr.Branches["release-1.0"].Status, cmd.BranchStatusPicked)
	}
}
