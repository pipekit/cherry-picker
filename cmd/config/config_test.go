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
		name               string
		configFile         string
		org                string
		repo               string
		sourceBranch       string
		aiAssistantCommand string
		fileExists         bool
		saveError          bool
		wantErr            bool
		wantErrMsg         string
	}{
		{
			name:               "successful init with defaults",
			configFile:         "test-config.yaml",
			org:                "testorg",
			repo:               "testrepo",
			sourceBranch:       "main",
			aiAssistantCommand: "cursor-agent",
			fileExists:         false,
			saveError:          false,
			wantErr:            false,
		},
		{
			name:               "successful init with custom source branch",
			configFile:         "test-config-2.yaml",
			org:                "testorg",
			repo:               "testrepo",
			sourceBranch:       "develop",
			aiAssistantCommand: "claude",
			fileExists:         false,
			saveError:          false,
			wantErr:            false,
		},
		{
			name:               "update existing config",
			configFile:         "existing-config.yaml",
			org:                "testorg",
			repo:               "testrepo",
			sourceBranch:       "main",
			aiAssistantCommand: "cursor-agent",
			fileExists:         true,
			saveError:          false,
			wantErr:            false,
		},
		{
			name:               "save config error",
			configFile:         "test-config-3.yaml",
			org:                "testorg",
			repo:               "testrepo",
			sourceBranch:       "main",
			aiAssistantCommand: "cursor-agent",
			fileExists:         false,
			saveError:          true,
			wantErr:            true,
			wantErrMsg:         "failed to save configuration: save error",
		},
		{
			name:               "partial update existing config",
			configFile:         "partial-update.yaml",
			org:                "", // Don't update org
			repo:               "newrepo",
			sourceBranch:       "", // Don't update source branch
			aiAssistantCommand: "cursor-agent",
			fileExists:         true,
			saveError:          false,
			wantErr:            false,
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
			saveConfig := func(_ string, config *cmd.Config) error {
				if tt.saveError {
					return fmt.Errorf("save error")
				}
				savedConfig = config
				return nil
			}

			// Mock load function
			loadConfig := func(_ string) (*cmd.Config, error) {
				if tt.fileExists {
					// Return existing config
					return &cmd.Config{
						Org:                "existingorg",
						Repo:               "existingrepo",
						SourceBranch:       "existing-branch",
						AIAssistantCommand: "existing-assistant",
					}, nil
				}
				// File doesn't exist
				return nil, fmt.Errorf("file not found")
			}

			// Run the function
			err := runConfig(configPath, tt.org, tt.repo, tt.sourceBranch, tt.aiAssistantCommand, loadConfig, saveConfig)

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
			}
		})
	}
}

func TestNewConfigCmd(t *testing.T) {
	// Mock functions
	loadConfig := func(_ string) (*cmd.Config, error) {
		return nil, fmt.Errorf("file not found")
	}

	var savedConfig *cmd.Config
	saveConfig := func(_ string, config *cmd.Config) error {
		savedConfig = config
		return nil
	}

	configFile := "test-config.yaml"
	cobraCmd := NewConfigCmd(&configFile, loadConfig, saveConfig)

	// Test command properties
	if cobraCmd.Use != "config" {
		t.Errorf("NewConfigCmd() Use = %v, want %v", cobraCmd.Use, "config")
	}

	if cobraCmd.Short != "Initialize a new cherry-picks.yaml configuration file" {
		t.Errorf("NewConfigCmd() Short = %v, want %v", cobraCmd.Short, "Initialize a new cherry-picks.yaml configuration file")
	}

	// Test flags (config flag should no longer be present as it's global)
	flags := cobraCmd.Flags()

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

	aiAssistantFlag := flags.Lookup("ai-assistant")
	if aiAssistantFlag == nil {
		t.Error("NewConfigCmd() missing ai-assistant flag")
	}

	// target-branches flag should no longer exist
	targetBranchesFlag := flags.Lookup("target-branches")
	if targetBranchesFlag != nil {
		t.Error("NewConfigCmd() should not have target-branches flag (branches determined from PR labels)")
	}

	// Test autodetection from git - this should succeed since we're in a git repo
	cobraCmd.SetArgs([]string{"--ai-assistant", "cursor-agent"})
	err := cobraCmd.Execute()
	if err != nil {
		t.Logf("NewConfigCmd() executed with autodetection: %v", err)
	}

	// Verify autodetection worked if we're in a git repo
	if savedConfig != nil {
		if savedConfig.Org == "" {
			t.Error("NewConfigCmd() autodetection should have set org from git remote")
		}
		if savedConfig.Repo == "" {
			t.Error("NewConfigCmd() autodetection should have set repo from git remote")
		}
		if savedConfig.SourceBranch == "" {
			t.Error("NewConfigCmd() autodetection should have set source branch from git or default to 'main'")
		}
		t.Logf("Autodetected: org=%s, repo=%s, source=%s", savedConfig.Org, savedConfig.Repo, savedConfig.SourceBranch)
	}
}
