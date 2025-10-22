package commands

import (
	"errors"
	"testing"

	"github.com/alan/cherry-picker/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseCommand_Init(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		loadConfig func(string) (*cmd.Config, error)
		wantErr    bool
	}{
		{
			name:  "successful init",
			token: "test-token",
			loadConfig: func(_ string) (*cmd.Config, error) {
				return &cmd.Config{
					Org:  "testorg",
					Repo: "testrepo",
				}, nil
			},
			wantErr: false,
		},
		{
			name:  "config load error",
			token: "test-token",
			loadConfig: func(_ string) (*cmd.Config, error) {
				return nil, errors.New("failed to load config")
			},
			wantErr: true,
		},
		{
			name:  "missing github token",
			token: "",
			loadConfig: func(_ string) (*cmd.Config, error) {
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
			t.Setenv("GITHUB_TOKEN", tt.token)

			configFile := "test-config.yaml"
			bc := &BaseCommand{
				ConfigFile: &configFile,
				LoadConfig: tt.loadConfig,
			}

			err := bc.Init(t.Context())

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, bc.Config)
				assert.NotNil(t, bc.GitHubClient)
			}
		})
	}
}

func TestBaseCommand_InitSetsFields(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	expectedConfig := &cmd.Config{
		Org:  "myorg",
		Repo: "myrepo",
	}

	configFile := "test-config.yaml"
	bc := &BaseCommand{
		ConfigFile: &configFile,
		LoadConfig: func(_ string) (*cmd.Config, error) {
			return expectedConfig, nil
		},
	}

	err := bc.Init(t.Context())

	require.NoError(t, err)
	assert.Equal(t, expectedConfig, bc.Config)
}
