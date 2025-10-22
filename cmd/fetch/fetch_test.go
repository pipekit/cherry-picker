package fetch

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alan/cherry-picker/cmd"
)

func TestNewFetchCmd(t *testing.T) {
	// Mock functions
	loadConfig := func(filename string) (*cmd.Config, error) {
		return nil, nil
	}
	saveConfig := func(filename string, config *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	cmd := NewFetchCmd(&configFile, loadConfig, saveConfig)

	// Test command properties
	if cmd.Use != "fetch" {
		t.Errorf("NewFetchCmd() Use = %v, want %v", cmd.Use, "fetch")
	}

	if cmd.Short != "Fetch new merged PRs from GitHub that need cherry-picking decisions" {
		t.Errorf("NewFetchCmd() Short = %v, want expected short description", cmd.Short)
	}

	// Test flags (config flag should no longer be present as it's global)
	flags := cmd.Flags()

	configFlag := flags.Lookup("config")
	if configFlag != nil {
		t.Error("NewFetchCmd() should not have local config flag (it's now global)")
	}

	sinceFlag := flags.Lookup("since")
	if sinceFlag == nil {
		t.Error("NewFetchCmd() missing since flag")
	}
}

func TestRunFetch_NoToken(t *testing.T) {
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

	// Test using the command structure
	configFile := "test-config.yaml"
	fetchCmd := &FetchCommand{}
	fetchCmd.ConfigFile = &configFile
	fetchCmd.LoadConfig = loadConfig
	fetchCmd.SaveConfig = saveConfig
	fetchCmd.SinceDate = ""

	err := fetchCmd.Init()

	if err == nil {
		t.Error("FetchCommand.Init() expected error for missing GITHUB_TOKEN, got nil")
	}

	if !strings.Contains(err.Error(), "GITHUB_TOKEN environment variable is required") {
		t.Errorf("FetchCommand.Init() error = %v, want error about GITHUB_TOKEN", err)
	}
}

func TestRunFetch_InvalidDateFormat(t *testing.T) {
	// Set a dummy token
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

	// Test using the command structure
	configFile := "test-config.yaml"
	fetchCmd := &FetchCommand{}
	fetchCmd.ConfigFile = &configFile
	fetchCmd.LoadConfig = loadConfig
	fetchCmd.SaveConfig = saveConfig
	fetchCmd.SinceDate = "invalid-date"

	if err := fetchCmd.Init(); err != nil {
		t.Fatalf("FetchCommand.Init() unexpected error: %v", err)
	}

	err := fetchCmd.Run()

	if err == nil {
		t.Error("FetchCommand.Run() expected error for invalid date format, got nil")
	}

	if !strings.Contains(err.Error(), "invalid date format") {
		t.Errorf("FetchCommand.Run() error = %v, want error about invalid date format", err)
	}
}

func TestRunFetch_LoadConfigError(t *testing.T) {
	os.Setenv("GITHUB_TOKEN", "test-token")
	defer os.Unsetenv("GITHUB_TOKEN")

	loadConfig := func(filename string) (*cmd.Config, error) {
		return nil, fmt.Errorf("config load error")
	}
	saveConfig := func(filename string, config *cmd.Config) error {
		return nil
	}

	// Test using the command structure
	configFile := "test-config.yaml"
	fetchCmd := &FetchCommand{}
	fetchCmd.ConfigFile = &configFile
	fetchCmd.LoadConfig = loadConfig
	fetchCmd.SaveConfig = saveConfig

	err := fetchCmd.Init()

	if err == nil {
		t.Error("FetchCommand.Init() expected error for config load failure, got nil")
	}

	if !strings.Contains(err.Error(), "config load error") {
		t.Errorf("FetchCommand.Init() error = %v, want error about config load failure", err)
	}
}

func TestDetermineSinceDate(t *testing.T) {
	// Test with explicit date string
	t.Run("Explicit date string", func(t *testing.T) {
		since, err := determineSinceDate("2024-01-15", nil)
		if err != nil {
			t.Fatalf("determineSinceDate() unexpected error: %v", err)
		}
		expected := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
		if !since.Equal(expected) {
			t.Errorf("determineSinceDate() = %v, want %v", since, expected)
		}
	})

	// Test with invalid date format
	t.Run("Invalid date format", func(t *testing.T) {
		_, err := determineSinceDate("invalid-date", nil)
		if err == nil {
			t.Error("determineSinceDate() expected error for invalid date, got nil")
		}
	})

	// Test with lastFetchDate
	t.Run("Use lastFetchDate", func(t *testing.T) {
		lastFetch := time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)
		since, err := determineSinceDate("", &lastFetch)
		if err != nil {
			t.Fatalf("determineSinceDate() unexpected error: %v", err)
		}
		if !since.Equal(lastFetch) {
			t.Errorf("determineSinceDate() = %v, want %v", since, lastFetch)
		}
	})

	// Test with no parameters - should default to 30 days ago
	t.Run("Default to 30 days ago", func(t *testing.T) {
		now := time.Now()
		since, err := determineSinceDate("", nil)
		if err != nil {
			t.Fatalf("determineSinceDate() unexpected error: %v", err)
		}
		expected := now.AddDate(0, 0, -30)
		// Check that it's approximately 30 days ago (within 1 second)
		if since.After(now) || since.Before(expected.Add(-1*time.Second)) {
			t.Errorf("determineSinceDate() = %v, want approximately %v", since, expected)
		}
	})

	// Test that explicit date takes precedence over lastFetchDate
	t.Run("Explicit date overrides lastFetchDate", func(t *testing.T) {
		lastFetch := time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)
		since, err := determineSinceDate("2024-03-01", &lastFetch)
		if err != nil {
			t.Fatalf("determineSinceDate() unexpected error: %v", err)
		}
		expected := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
		if !since.Equal(expected) {
			t.Errorf("determineSinceDate() = %v, want %v (should use explicit date)", since, expected)
		}
	})
}
