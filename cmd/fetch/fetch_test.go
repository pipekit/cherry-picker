package fetch

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alan/cherry-picker/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewFetchCmd tests command creation and initialization
func TestNewFetchCmd(t *testing.T) {
	loadConfig := func(_ string) (*cmd.Config, error) {
		return &cmd.Config{}, nil
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	cobraCmd := NewFetchCmd(&configFile, loadConfig, saveConfig)

	assert.NotNil(t, cobraCmd)
	assert.Equal(t, "fetch", cobraCmd.Use)
	assert.Equal(t, "Fetch new merged PRs from GitHub that need cherry-picking decisions", cobraCmd.Short)
	assert.NotEmpty(t, cobraCmd.Long)
	assert.NotNil(t, cobraCmd.RunE)

	// Test flags
	sinceFlag := cobraCmd.Flags().Lookup("since")
	assert.NotNil(t, sinceFlag, "should have since flag")

	// Config flag should not be present as it's global
	configFlag := cobraCmd.Flags().Lookup("config")
	assert.Nil(t, configFlag, "should not have local config flag (it's global)")
}

// TestFetchCmd_RunE_InvalidSinceDate tests error handling for invalid since date
func TestFetchCmd_RunE_InvalidSinceDate(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	loadConfig := func(_ string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:  "test-org",
			Repo: "test-repo",
		}, nil
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	cobraCmd := NewFetchCmd(&configFile, loadConfig, saveConfig)

	// Set an invalid since date flag
	err := cobraCmd.Flags().Set("since", "invalid-date")
	require.NoError(t, err)

	err = cobraCmd.RunE(cobraCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid date format")
}

// TestFetchCmd_RunE_ConfigLoadError tests error when config fails to load
func TestFetchCmd_RunE_ConfigLoadError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	loadConfig := func(_ string) (*cmd.Config, error) {
		return nil, errors.New("config load error")
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	cobraCmd := NewFetchCmd(&configFile, loadConfig, saveConfig)

	err := cobraCmd.RunE(cobraCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config load error")
}

// TestCommand_Init_NoToken tests initialization without GitHub token
func TestCommand_Init_NoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")

	loadConfig := func(_ string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:                "testorg",
			AIAssistantCommand: "cursor-agent",
			Repo:               "testrepo",
			SourceBranch:       "main",
		}, nil
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	fetchCmd := &command{}
	fetchCmd.ConfigFile = &configFile
	fetchCmd.LoadConfig = loadConfig
	fetchCmd.SaveConfig = saveConfig
	fetchCmd.SinceDate = ""

	err := fetchCmd.Init(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GITHUB_TOKEN environment variable is required")
}

// TestCommand_Run_InvalidDateFormat tests Run with invalid date format
func TestCommand_Run_InvalidDateFormat(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	loadConfig := func(_ string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:                "testorg",
			AIAssistantCommand: "cursor-agent",
			Repo:               "testrepo",
			SourceBranch:       "main",
		}, nil
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	fetchCmd := &command{}
	fetchCmd.ConfigFile = &configFile
	fetchCmd.LoadConfig = loadConfig
	fetchCmd.SaveConfig = saveConfig
	fetchCmd.SinceDate = "invalid-date"

	err := fetchCmd.Init(t.Context())
	require.NoError(t, err)

	err = fetchCmd.Run(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid date format")
}

// TestCommand_Init_LoadConfigError tests Init with config load failure
func TestCommand_Init_LoadConfigError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	loadConfig := func(_ string) (*cmd.Config, error) {
		return nil, fmt.Errorf("config load error")
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	fetchCmd := &command{}
	fetchCmd.ConfigFile = &configFile
	fetchCmd.LoadConfig = loadConfig
	fetchCmd.SaveConfig = saveConfig

	err := fetchCmd.Init(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config load error")
}

// TestDetermineSinceDate tests the date determination logic
func TestDetermineSinceDate(t *testing.T) {
	tests := []struct {
		name          string
		sinceDate     string
		lastFetchDate *time.Time
		wantErr       bool
		checkResult   func(*testing.T, time.Time)
	}{
		{
			name:      "explicit date string",
			sinceDate: "2024-01-15",
			wantErr:   false,
			checkResult: func(t *testing.T, result time.Time) {
				t.Helper()
				expected := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
				assert.Equal(t, expected, result)
			},
		},
		{
			name:      "invalid date format",
			sinceDate: "invalid-date",
			wantErr:   true,
		},
		{
			name:          "use lastFetchDate",
			sinceDate:     "",
			lastFetchDate: ptrTime(time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)),
			wantErr:       false,
			checkResult: func(t *testing.T, result time.Time) {
				t.Helper()
				expected := time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)
				assert.Equal(t, expected, result)
			},
		},
		{
			name:      "default to 30 days ago",
			sinceDate: "",
			wantErr:   false,
			checkResult: func(t *testing.T, result time.Time) {
				t.Helper()
				now := time.Now()
				expected := now.AddDate(0, 0, -30)
				// Check that it's approximately 30 days ago (within 1 minute)
				assert.WithinDuration(t, expected, result, time.Minute)
				assert.True(t, result.Before(now))
			},
		},
		{
			name:          "explicit date overrides lastFetchDate",
			sinceDate:     "2024-03-01",
			lastFetchDate: ptrTime(time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)),
			wantErr:       false,
			checkResult: func(t *testing.T, result time.Time) {
				t.Helper()
				expected := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
				assert.Equal(t, expected, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := determineSinceDate(tt.sinceDate, tt.lastFetchDate)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.checkResult != nil {
					tt.checkResult(t, result)
				}
			}
		})
	}
}

// TestCommandOutput tests command output formatting
func TestCommandOutput(t *testing.T) {
	configFile := "test-config.yaml"
	cobraCmd := NewFetchCmd(&configFile, nil, nil)

	// Test help output
	var buf bytes.Buffer
	cobraCmd.SetOut(&buf)
	cobraCmd.SetErr(&buf)

	err := cobraCmd.Help()
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String(), "should generate help text")
}

// ptrTime is a helper to create time.Time pointers
func ptrTime(t time.Time) *time.Time {
	return &t
}
