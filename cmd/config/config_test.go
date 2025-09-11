package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alan/cherry-picker/cmd"
)

func TestRunConfig(t *testing.T) {
	tests := []struct {
		name           string
		configFile     string
		org            string
		repo           string
		sourceBranch   string
		targetBranches []string
		fileExists     bool
		saveError      bool
		wantErr        bool
		wantErrMsg     string
	}{
		{
			name:           "successful init with defaults",
			configFile:     "test-config.yaml",
			org:            "testorg",
			repo:           "testrepo",
			sourceBranch:   "main",
			targetBranches: []string{},
			fileExists:     false,
			saveError:      false,
			wantErr:        false,
		},
		{
			name:           "successful init with target branches",
			configFile:     "test-config-2.yaml",
			org:            "testorg",
			repo:           "testrepo",
			sourceBranch:   "develop",
			targetBranches: []string{"release-1.0", "release-2.0"},
			fileExists:     false,
			saveError:      false,
			wantErr:        false,
		},
		{
			name:           "update existing config",
			configFile:     "existing-config.yaml",
			org:            "testorg",
			repo:           "testrepo",
			sourceBranch:   "main",
			targetBranches: []string{"release-3.0"},
			fileExists:     true,
			saveError:      false,
			wantErr:        false,
		},
		{
			name:           "save config error",
			configFile:     "test-config-3.yaml",
			org:            "testorg",
			repo:           "testrepo",
			sourceBranch:   "main",
			targetBranches: []string{},
			fileExists:     false,
			saveError:      true,
			wantErr:        true,
			wantErrMsg:     "failed to save configuration: save error",
		},
		{
			name:           "partial update existing config",
			configFile:     "partial-update.yaml",
			org:            "", // Don't update org
			repo:           "newrepo",
			sourceBranch:   "", // Don't update source branch
			targetBranches: []string{"release-4.0", "release-5.0"},
			fileExists:     true,
			saveError:      false,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, tt.configFile)

			// Create file if it should exist
			if tt.fileExists {
				if err := os.WriteFile(configPath, []byte("existing"), 0644); err != nil {
					t.Fatalf("failed to create existing file: %v", err)
				}
			}

			// Mock save function
			var savedConfig *cmd.Config
			saveConfig := func(filename string, config *cmd.Config) error {
				if tt.saveError {
					return fmt.Errorf("save error")
				}
				savedConfig = config
				return nil
			}

			// Mock load function
			loadConfig := func(filename string) (*cmd.Config, error) {
				if tt.fileExists {
					// Return existing config
					return &cmd.Config{
						Org:            "existingorg",
						Repo:           "existingrepo",
						SourceBranch:   "existing-branch",
						TargetBranches: []string{"existing-target"},
					}, nil
				}
				// File doesn't exist
				return nil, fmt.Errorf("file not found")
			}

			// Run the function
			err := runConfig(configPath, tt.org, tt.repo, tt.sourceBranch, tt.targetBranches, loadConfig, saveConfig)

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Errorf("runConfig() expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("runConfig() error = %v, want error containing %v", err, tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("runConfig() unexpected error = %v", err)
				return
			}

			// Verify saved config
			if savedConfig == nil {
				t.Error("runConfig() did not save config")
				return
			}

			// For partial updates, verify the merge behavior
			if tt.name == "partial update existing config" {
				// Org should be preserved from existing config (not updated since tt.org is empty)
				if savedConfig.Org != "existingorg" {
					t.Errorf("runConfig() saved org = %v, want %v (preserved from existing)", savedConfig.Org, "existingorg")
				}
				// Repo should be updated
				if savedConfig.Repo != "newrepo" {
					t.Errorf("runConfig() saved repo = %v, want %v (updated)", savedConfig.Repo, "newrepo")
				}
				// Source branch should be preserved (not updated since tt.sourceBranch is empty)
				if savedConfig.SourceBranch != "existing-branch" {
					t.Errorf("runConfig() saved sourceBranch = %v, want %v (preserved from existing)", savedConfig.SourceBranch, "existing-branch")
				}
				// Target branches should be updated
				if len(savedConfig.TargetBranches) != len(tt.targetBranches) {
					t.Errorf("runConfig() saved targetBranches length = %v, want %v (updated)", len(savedConfig.TargetBranches), len(tt.targetBranches))
				}
				for i, branch := range tt.targetBranches {
					if i >= len(savedConfig.TargetBranches) || savedConfig.TargetBranches[i] != branch {
						t.Errorf("runConfig() saved targetBranches[%d] = %v, want %v (updated)", i, savedConfig.TargetBranches[i], branch)
					}
				}
			} else {
				// Normal validation for other test cases
				if savedConfig.Org != tt.org {
					t.Errorf("runConfig() saved org = %v, want %v", savedConfig.Org, tt.org)
				}
				if savedConfig.Repo != tt.repo {
					t.Errorf("runConfig() saved repo = %v, want %v", savedConfig.Repo, tt.repo)
				}
				if savedConfig.SourceBranch != tt.sourceBranch {
					t.Errorf("runConfig() saved sourceBranch = %v, want %v", savedConfig.SourceBranch, tt.sourceBranch)
				}
				if len(savedConfig.TargetBranches) != len(tt.targetBranches) {
					t.Errorf("runConfig() saved targetBranches length = %v, want %v", len(savedConfig.TargetBranches), len(tt.targetBranches))
				}
				for i, branch := range tt.targetBranches {
					if i >= len(savedConfig.TargetBranches) || savedConfig.TargetBranches[i] != branch {
						t.Errorf("runConfig() saved targetBranches[%d] = %v, want %v", i, savedConfig.TargetBranches[i], branch)
					}
				}
			}
		})
	}
}

func TestNewConfigCmd(t *testing.T) {
	// Mock functions
	loadConfig := func(filename string) (*cmd.Config, error) {
		return nil, fmt.Errorf("file not found")
	}
	saveConfig := func(filename string, config *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	cmd := NewConfigCmd(&configFile, loadConfig, saveConfig)

	// Test command properties
	if cmd.Use != "config" {
		t.Errorf("NewConfigCmd() Use = %v, want %v", cmd.Use, "config")
	}

	if cmd.Short != "Initialize a new cherry-picks.yaml configuration file" {
		t.Errorf("NewConfigCmd() Short = %v, want %v", cmd.Short, "Initialize a new cherry-picks.yaml configuration file")
	}

	// Test flags (config flag should no longer be present as it's global)
	flags := cmd.Flags()

	configFlag := flags.Lookup("config")
	if configFlag != nil {
		t.Error("NewConfigCmd() should not have local config flag (it's now global)")
	}

	orgFlag := flags.Lookup("org")
	if orgFlag == nil {
		t.Error("NewConfigCmd() missing org flag")
	}

	repoFlag := flags.Lookup("repo")
	if repoFlag == nil {
		t.Error("NewConfigCmd() missing repo flag")
	}

	sourceBranchFlag := flags.Lookup("source-branch")
	if sourceBranchFlag == nil {
		t.Error("NewConfigCmd() missing source-branch flag")
	} else if sourceBranchFlag.DefValue != "" {
		t.Errorf("NewConfigCmd() source-branch flag default = %v, want %v (empty for auto-detection)", sourceBranchFlag.DefValue, "")
	}

	targetBranchesFlag := flags.Lookup("target-branches")
	if targetBranchesFlag == nil {
		t.Error("NewConfigCmd() missing target-branches flag")
	}

	// Note: org and repo are no longer required flags since they can be auto-detected from git
	// Test that command fails gracefully when org/repo cannot be determined
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("NewConfigCmd() should fail when org/repo cannot be determined")
	}
}
