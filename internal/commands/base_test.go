package commands

import (
	"errors"
	"os"
	"testing"

	"github.com/alan/cherry-picker/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseCommand_Init(t *testing.T) {
	tests := []struct {
		name        string
		setupEnv    func() (cleanup func())
		loadConfig  func(string) (*cmd.Config, error)
		wantErr     bool
	}{
		{
			name: "successful init",
			setupEnv: func() func() {
				oldToken := os.Getenv("GITHUB_TOKEN")
				os.Setenv("GITHUB_TOKEN", "test-token")
				return func() {
					if oldToken != "" {
						os.Setenv("GITHUB_TOKEN", oldToken)
					} else {
						os.Unsetenv("GITHUB_TOKEN")
					}
				}
			},
			loadConfig: func(path string) (*cmd.Config, error) {
				return &cmd.Config{
					Org:  "testorg",
					Repo: "testrepo",
				}, nil
			},
			wantErr: false,
		},
		{
			name: "config load error",
			setupEnv: func() func() {
				oldToken := os.Getenv("GITHUB_TOKEN")
				os.Setenv("GITHUB_TOKEN", "test-token")
				return func() {
					if oldToken != "" {
						os.Setenv("GITHUB_TOKEN", oldToken)
					} else {
						os.Unsetenv("GITHUB_TOKEN")
					}
				}
			},
			loadConfig: func(path string) (*cmd.Config, error) {
				return nil, errors.New("failed to load config")
			},
			wantErr: true,
		},
		{
			name: "missing github token",
			setupEnv: func() func() {
				oldToken := os.Getenv("GITHUB_TOKEN")
				os.Unsetenv("GITHUB_TOKEN")
				return func() {
					if oldToken != "" {
						os.Setenv("GITHUB_TOKEN", oldToken)
					}
				}
			},
			loadConfig: func(path string) (*cmd.Config, error) {
				return &cmd.Config{
					Org:  "testorg",
					Repo: "testrepo",
				}, nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupEnv()
			defer cleanup()

			configFile := "test-config.yaml"
			bc := &BaseCommand{
				ConfigFile: &configFile,
				LoadConfig: tt.loadConfig,
			}

			err := bc.Init()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, bc.Config)
				assert.NotNil(t, bc.GitHubClient)
				assert.NotNil(t, bc.Context)
			}
		})
	}
}

func TestBaseCommand_InitSetsFields(t *testing.T) {
	oldToken := os.Getenv("GITHUB_TOKEN")
	os.Setenv("GITHUB_TOKEN", "test-token")
	defer func() {
		if oldToken != "" {
			os.Setenv("GITHUB_TOKEN", oldToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN")
		}
	}()

	expectedConfig := &cmd.Config{
		Org:  "myorg",
		Repo: "myrepo",
	}

	configFile := "test-config.yaml"
	bc := &BaseCommand{
		ConfigFile: &configFile,
		LoadConfig: func(path string) (*cmd.Config, error) {
			return expectedConfig, nil
		},
	}

	err := bc.Init()

	require.NoError(t, err)
	assert.Equal(t, expectedConfig, bc.Config)
}
