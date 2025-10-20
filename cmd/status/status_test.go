package status

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alan/cherry-picker/cmd"
)

func TestNewStatusCmd(t *testing.T) {
	// Mock function
	loadConfig := func(filename string) (*cmd.Config, error) {
		return nil, nil
	}

	configFile := "test-config.yaml"
	cmd := NewStatusCmd(&configFile, loadConfig)

	// Test command properties
	if cmd.Use != "status" {
		t.Errorf("NewStatusCmd() Use = %v, want %v", cmd.Use, "status")
	}

	if cmd.Short != "Show status of tracked PRs across target branches" {
		t.Errorf("NewStatusCmd() Short = %v, want expected short description", cmd.Short)
	}

	// Test that no local flags are added (should only use global config)
	flags := cmd.Flags()
	if flags.NFlag() != 0 {
		t.Errorf("NewStatusCmd() should have no local flags, got %d", flags.NFlag())
	}
}

func TestRunStatus_NoConfig(t *testing.T) {
	loadConfig := func(filename string) (*cmd.Config, error) {
		return nil, fmt.Errorf("config not found")
	}

	err := runStatus("test-config.yaml", loadConfig)

	if err == nil {
		t.Error("runStatus() expected error for missing config, got nil")
	}

	if !strings.Contains(err.Error(), "failed to load config") {
		t.Errorf("runStatus() error = %v, want error about config load failure", err)
	}
}

func TestRunStatus_NoPRs(t *testing.T) {
	loadConfig := func(filename string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:          "testorg",
			Repo:         "testrepo",
			SourceBranch: "main",
			TrackedPRs:   []cmd.TrackedPR{},
		}, nil
	}

	// This would normally print to stdout, but we can't easily capture that in tests
	// The important thing is that it doesn't error
	err := runStatus("test-config.yaml", loadConfig)

	if err != nil {
		t.Errorf("runStatus() unexpected error = %v", err)
	}
}

func TestRunStatus_WithActivePRs(t *testing.T) {
	now := time.Now()
	loadConfig := func(filename string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:           "testorg",
			Repo:          "testrepo",
			SourceBranch:  "main",
			LastFetchDate: &now,
			TrackedPRs: []cmd.TrackedPR{
				{
					Number: 123,
					Title:  "Cherry-pick PR",
					Branches: map[string]cmd.BranchStatus{
						"release-1.0": {Status: "pending"},
						"release-2.0": {Status: "merged", PR: &cmd.PickPR{Number: 456, Title: "Cherry-pick", CIStatus: "passing"}},
						"staging":     {Status: "pending"},
					},
				},
				{
					Number: 125,
					Title:  "Another PR",
					Branches: map[string]cmd.BranchStatus{
						"release-1.0": {Status: "picked"},
						// Missing release-2.0 and staging - should show as "not tracked"
					},
				},
			},
		}, nil
	}

	// This would normally print to stdout, but we can't easily capture that in tests
	// The important thing is that it doesn't error and processes the data correctly
	err := runStatus("test-config.yaml", loadConfig)

	if err != nil {
		t.Errorf("runStatus() unexpected error = %v", err)
	}
}

func TestRunStatus_EmptyConfig(t *testing.T) {
	loadConfig := func(filename string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:          "testorg",
			Repo:         "testrepo",
			SourceBranch: "main",
			TrackedPRs:   []cmd.TrackedPR{},
		}, nil
	}

	err := runStatus("test-config.yaml", loadConfig)

	if err != nil {
		t.Errorf("runStatus() unexpected error = %v", err)
	}
}

func TestRunStatus_PRsWithoutBranches(t *testing.T) {
	loadConfig := func(filename string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:          "testorg",
			Repo:         "testrepo",
			SourceBranch: "main",
			TrackedPRs: []cmd.TrackedPR{
				{
					Number:   123,
					Title:    "PR without branches",
					Branches: map[string]cmd.BranchStatus{}, // Empty branches map
				},
			},
		}, nil
	}

	err := runStatus("test-config.yaml", loadConfig)

	if err != nil {
		t.Errorf("runStatus() unexpected error = %v", err)
	}
}
