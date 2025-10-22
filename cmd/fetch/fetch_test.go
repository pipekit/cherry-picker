package fetch

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alan/cherry-picker/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewFetchCmd tests command creation and initialization
func TestNewFetchCmd(t *testing.T) {
	loadConfig := func(filename string) (*cmd.Config, error) {
		return &cmd.Config{}, nil
	}
	saveConfig := func(filename string, config *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	cmd := NewFetchCmd(&configFile, loadConfig, saveConfig)

	assert.NotNil(t, cmd)
	assert.Equal(t, "fetch", cmd.Use)
	assert.Equal(t, "Fetch new merged PRs from GitHub that need cherry-picking decisions", cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotNil(t, cmd.RunE)

	// Test flags
	sinceFlag := cmd.Flags().Lookup("since")
	assert.NotNil(t, sinceFlag, "should have since flag")

	// Config flag should not be present as it's global
	configFlag := cmd.Flags().Lookup("config")
	assert.Nil(t, configFlag, "should not have local config flag (it's global)")
}

// TestFetchCmd_RunE_InvalidSinceDate tests error handling for invalid since date
func TestFetchCmd_RunE_InvalidSinceDate(t *testing.T) {
	os.Setenv("GITHUB_TOKEN", "test-token")
	defer os.Unsetenv("GITHUB_TOKEN")

	loadConfig := func(path string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:  "test-org",
			Repo: "test-repo",
		}, nil
	}
	saveConfig := func(path string, config *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	cmd := NewFetchCmd(&configFile, loadConfig, saveConfig)

	// Set an invalid since date flag
	err := cmd.Flags().Set("since", "invalid-date")
	require.NoError(t, err)

	err = cmd.RunE(cmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid date format")
}

// TestFetchCmd_RunE_ConfigLoadError tests error when config fails to load
func TestFetchCmd_RunE_ConfigLoadError(t *testing.T) {
	os.Setenv("GITHUB_TOKEN", "test-token")
	defer os.Unsetenv("GITHUB_TOKEN")

	loadConfig := func(path string) (*cmd.Config, error) {
		return nil, errors.New("config load error")
	}
	saveConfig := func(path string, config *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	cmd := NewFetchCmd(&configFile, loadConfig, saveConfig)

	err := cmd.RunE(cmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config load error")
}

// TestFetchCommand_Init_NoToken tests initialization without GitHub token
func TestFetchCommand_Init_NoToken(t *testing.T) {
	// Temporarily unset GITHUB_TOKEN
	originalToken := os.Getenv("GITHUB_TOKEN")
	os.Unsetenv("GITHUB_TOKEN")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITHUB_TOKEN", originalToken)
		}
	}()

	loadConfig := func(filename string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:                "testorg",
			AIAssistantCommand: "cursor-agent",
			Repo:               "testrepo",
			SourceBranch:       "main",
		}, nil
	}
	saveConfig := func(filename string, config *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	fetchCmd := &FetchCommand{}
	fetchCmd.ConfigFile = &configFile
	fetchCmd.LoadConfig = loadConfig
	fetchCmd.SaveConfig = saveConfig
	fetchCmd.SinceDate = ""

	err := fetchCmd.Init()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GITHUB_TOKEN environment variable is required")
}

// TestFetchCommand_Run_InvalidDateFormat tests Run with invalid date format
func TestFetchCommand_Run_InvalidDateFormat(t *testing.T) {
	os.Setenv("GITHUB_TOKEN", "test-token")
	defer os.Unsetenv("GITHUB_TOKEN")

	loadConfig := func(filename string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:                "testorg",
			AIAssistantCommand: "cursor-agent",
			Repo:               "testrepo",
			SourceBranch:       "main",
		}, nil
	}
	saveConfig := func(filename string, config *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	fetchCmd := &FetchCommand{}
	fetchCmd.ConfigFile = &configFile
	fetchCmd.LoadConfig = loadConfig
	fetchCmd.SaveConfig = saveConfig
	fetchCmd.SinceDate = "invalid-date"

	err := fetchCmd.Init()
	require.NoError(t, err)

	err = fetchCmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid date format")
}

// TestFetchCommand_Init_LoadConfigError tests Init with config load failure
func TestFetchCommand_Init_LoadConfigError(t *testing.T) {
	os.Setenv("GITHUB_TOKEN", "test-token")
	defer os.Unsetenv("GITHUB_TOKEN")

	loadConfig := func(filename string) (*cmd.Config, error) {
		return nil, fmt.Errorf("config load error")
	}
	saveConfig := func(filename string, config *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	fetchCmd := &FetchCommand{}
	fetchCmd.ConfigFile = &configFile
	fetchCmd.LoadConfig = loadConfig
	fetchCmd.SaveConfig = saveConfig

	err := fetchCmd.Init()
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
				expected := time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)
				assert.Equal(t, expected, result)
			},
		},
		{
			name:      "default to 30 days ago",
			sinceDate: "",
			wantErr:   false,
			checkResult: func(t *testing.T, result time.Time) {
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

// TestFetchCommandOutput tests command output formatting
func TestFetchCommandOutput(t *testing.T) {
	configFile := "test-config.yaml"
	cmd := NewFetchCmd(&configFile, nil, nil)

	// Test help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Help()
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String(), "should generate help text")
}

// ptrTime is a helper to create time.Time pointers
func ptrTime(t time.Time) *time.Time {
	return &t
}
