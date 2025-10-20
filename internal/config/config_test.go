package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alan/cherry-picker/cmd"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name         string
		fileContent  string
		wantErr      bool
		wantErrMsg   string
		expectedOrg  string
		expectedRepo string
	}{
		{
			name: "valid config",
			fileContent: `org: testorg
repo: testrepo
source_branch: main
target_branches:
  - release-1.0
  - release-2.0`,
			wantErr:      false,
			expectedOrg:  "testorg",
			expectedRepo: "testrepo",
		},
		{
			name: "minimal config",
			fileContent: `org: minimalorg
repo: minimalrepo
source_branch: develop
target_branches: []`,
			wantErr:      false,
			expectedOrg:  "minimalorg",
			expectedRepo: "minimalrepo",
		},
		{
			name:        "file not found",
			fileContent: "",
			wantErr:     true,
			wantErrMsg:  "failed to read config file",
		},
		{
			name:        "invalid yaml",
			fileContent: "invalid: yaml: content: [",
			wantErr:     true,
			wantErrMsg:  "failed to parse config file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "config.yaml")

			if tt.name != "file not found" {
				if err := os.WriteFile(configFile, []byte(tt.fileContent), 0644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
			}

			config, err := LoadConfig(configFile)

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadConfig() expected error, got nil")
					return
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("LoadConfig() error = %v, want error containing %v", err, tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("LoadConfig() unexpected error = %v", err)
				return
			}

			if config.Org != tt.expectedOrg {
				t.Errorf("LoadConfig() org = %v, want %v", config.Org, tt.expectedOrg)
			}

			if config.Repo != tt.expectedRepo {
				t.Errorf("LoadConfig() repo = %v, want %v", config.Repo, tt.expectedRepo)
			}
		})
	}
}

func TestSaveConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *cmd.Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &cmd.Config{
				Org:          "testorg",
				Repo:         "testrepo",
				SourceBranch: "main",
			},
			wantErr: false,
		},
		{
			name: "config with different source branch",
			config: &cmd.Config{
				Org:          "testorg",
				Repo:         "testrepo",
				SourceBranch: "develop",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "config.yaml")

			err := SaveConfig(configFile, tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SaveConfig() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("SaveConfig() unexpected error = %v", err)
				return
			}

			// Verify the file was created and can be loaded back
			loadedConfig, err := LoadConfig(configFile)
			if err != nil {
				t.Errorf("SaveConfig() created invalid file: %v", err)
				return
			}

			if loadedConfig.Org != tt.config.Org {
				t.Errorf("SaveConfig() saved org = %v, want %v", loadedConfig.Org, tt.config.Org)
			}

			if loadedConfig.Repo != tt.config.Repo {
				t.Errorf("SaveConfig() saved repo = %v, want %v", loadedConfig.Repo, tt.config.Repo)
			}

			if loadedConfig.SourceBranch != tt.config.SourceBranch {
				t.Errorf("SaveConfig() saved source_branch = %v, want %v", loadedConfig.SourceBranch, tt.config.SourceBranch)
			}
		})
	}
}
