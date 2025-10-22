package commands

import (
	"os"
	"testing"

	"github.com/alan/cherry-picker/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitializeGitHubClient(t *testing.T) {
	tests := []struct {
		name       string
		setupToken func() (cleanup func())
		config     *cmd.Config
		wantErr    bool
	}{
		{
			name: "successful initialization",
			setupToken: func() func() {
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
			config: &cmd.Config{
				Org:  "testorg",
				Repo: "testrepo",
			},
			wantErr: false,
		},
		{
			name: "missing GITHUB_TOKEN",
			setupToken: func() func() {
				oldToken := os.Getenv("GITHUB_TOKEN")
				os.Unsetenv("GITHUB_TOKEN")
				return func() {
					if oldToken != "" {
						os.Setenv("GITHUB_TOKEN", oldToken)
					}
				}
			},
			config: &cmd.Config{
				Org:  "testorg",
				Repo: "testrepo",
			},
			wantErr: true,
		},
		{
			name: "empty org and repo",
			setupToken: func() func() {
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
			config: &cmd.Config{
				Org:  "",
				Repo: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupToken()
			defer cleanup()

			client, ctx, err := InitializeGitHubClient(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, client)
				assert.Nil(t, ctx)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, ctx)
			}
		})
	}
}
