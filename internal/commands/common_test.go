package commands

import (
	"testing"

	"github.com/alan/cherry-picker/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitializeGitHubClient(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		config  *cmd.Config
		wantErr bool
	}{
		{
			name:  "successful initialization",
			token: "test-token",
			config: &cmd.Config{
				Org:  "testorg",
				Repo: "testrepo",
			},
			wantErr: false,
		},
		{
			name:  "missing GITHUB_TOKEN",
			token: "",
			config: &cmd.Config{
				Org:  "testorg",
				Repo: "testrepo",
			},
			wantErr: true,
		},
		{
			name:  "empty org and repo",
			token: "test-token",
			config: &cmd.Config{
				Org:  "",
				Repo: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", tt.token)

			client, ctx, err := InitializeGitHubClient(t.Context(), tt.config)

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
