package status

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alan/cherry-picker/cmd"
)

func TestNewStatusCmd(t *testing.T) {
	// Mock functions
	loadConfig := func(_ string) (*cmd.Config, error) {
		return nil, nil
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	configFile := "test-config.yaml"
	cobraCmd := NewStatusCmd(&configFile, loadConfig, saveConfig)

	// Test command properties
	if cobraCmd.Use != "status" {
		t.Errorf("NewStatusCmd() Use = %v, want %v", cobraCmd.Use, "status")
	}

	if cobraCmd.Short != "Show status of tracked PRs across target branches" {
		t.Errorf("NewStatusCmd() Short = %v, want expected short description", cobraCmd.Short)
	}

	// Test that flags are added
	flags := cobraCmd.Flags()
	if flags.Lookup("show-released") == nil {
		t.Error("NewStatusCmd() should have --show-released flag")
	}
	if flags.Lookup("fetch") == nil {
		t.Error("NewStatusCmd() should have --fetch flag")
	}
}

func TestRunStatus_NoConfig(t *testing.T) {
	loadConfig := func(_ string) (*cmd.Config, error) {
		return nil, fmt.Errorf("config not found")
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	err := runStatus(context.Background(), "test-config.yaml", loadConfig, saveConfig, false, false)

	if err == nil {
		t.Error("runStatus() expected error for missing config, got nil")
	}

	if !strings.Contains(err.Error(), "failed to load config") {
		t.Errorf("runStatus() error = %v, want error about config load failure", err)
	}
}

func TestRunStatus_NoPRs(t *testing.T) {
	loadConfig := func(_ string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:                "testorg",
			AIAssistantCommand: "cursor-agent",
			Repo:               "testrepo",
			SourceBranch:       "main",
			TrackedPRs:         []cmd.TrackedPR{},
		}, nil
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	// This would normally print to stdout, but we can't easily capture that in tests
	// The important thing is that it doesn't error
	err := runStatus(context.Background(), "test-config.yaml", loadConfig, saveConfig, false, false)

	if err != nil {
		t.Errorf("runStatus() unexpected error = %v", err)
	}
}

func TestRunStatus_WithActivePRs(t *testing.T) {
	now := time.Now()
	loadConfig := func(_ string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:                "testorg",
			Repo:               "testrepo",
			SourceBranch:       "main",
			AIAssistantCommand: "cursor-agent",
			LastFetchDate:      &now,
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
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}
	err := runStatus(context.Background(), "test-config.yaml", loadConfig, saveConfig, false, false)

	if err != nil {
		t.Errorf("runStatus() unexpected error = %v", err)
	}
}

func TestRunStatus_EmptyConfig(t *testing.T) {
	loadConfig := func(_ string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:                "testorg",
			AIAssistantCommand: "cursor-agent",
			Repo:               "testrepo",
			SourceBranch:       "main",
			TrackedPRs:         []cmd.TrackedPR{},
		}, nil
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	err := runStatus(context.Background(), "test-config.yaml", loadConfig, saveConfig, false, false)

	if err != nil {
		t.Errorf("runStatus() unexpected error = %v", err)
	}
}

func TestRunStatus_PRsWithoutBranches(t *testing.T) {
	loadConfig := func(_ string) (*cmd.Config, error) {
		return &cmd.Config{
			Org:                "testorg",
			AIAssistantCommand: "cursor-agent",
			Repo:               "testrepo",
			SourceBranch:       "main",
			TrackedPRs: []cmd.TrackedPR{
				{
					Number:   123,
					Title:    "PR without branches",
					Branches: map[string]cmd.BranchStatus{}, // Empty branches map
				},
			},
		}, nil
	}
	saveConfig := func(_ string, _ *cmd.Config) error {
		return nil
	}

	err := runStatus(context.Background(), "test-config.yaml", loadConfig, saveConfig, false, false)

	if err != nil {
		t.Errorf("runStatus() unexpected error = %v", err)
	}
}

func TestIsCompletelyReleased(t *testing.T) {
	tests := []struct {
		name string
		pr   cmd.TrackedPR
		want bool
	}{
		{
			name: "all branches released",
			pr: cmd.TrackedPR{
				Number: 123,
				Branches: map[string]cmd.BranchStatus{
					"release-3.6": {Status: cmd.BranchStatusReleased},
					"release-3.7": {Status: cmd.BranchStatusReleased},
				},
			},
			want: true,
		},
		{
			name: "one branch not released",
			pr: cmd.TrackedPR{
				Number: 123,
				Branches: map[string]cmd.BranchStatus{
					"release-3.6": {Status: cmd.BranchStatusReleased},
					"release-3.7": {Status: cmd.BranchStatusMerged},
				},
			},
			want: false,
		},
		{
			name: "no branches",
			pr: cmd.TrackedPR{
				Number:   123,
				Branches: map[string]cmd.BranchStatus{},
			},
			want: false,
		},
		{
			name: "mixed statuses",
			pr: cmd.TrackedPR{
				Number: 123,
				Branches: map[string]cmd.BranchStatus{
					"release-3.6": {Status: cmd.BranchStatusPending},
					"release-3.7": {Status: cmd.BranchStatusReleased},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCompletelyReleased(tt.pr)
			if got != tt.want {
				t.Errorf("isCompletelyReleased() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterNonReleasedPRs(t *testing.T) {
	prs := []cmd.TrackedPR{
		{
			Number: 100,
			Branches: map[string]cmd.BranchStatus{
				"release-3.6": {Status: cmd.BranchStatusReleased},
				"release-3.7": {Status: cmd.BranchStatusReleased},
			},
		},
		{
			Number: 200,
			Branches: map[string]cmd.BranchStatus{
				"release-3.6": {Status: cmd.BranchStatusMerged},
				"release-3.7": {Status: cmd.BranchStatusReleased},
			},
		},
		{
			Number: 300,
			Branches: map[string]cmd.BranchStatus{
				"release-3.6": {Status: cmd.BranchStatusPending},
			},
		},
	}

	filtered := filterNonReleasedPRs(prs)

	if len(filtered) != 2 {
		t.Errorf("filterNonReleasedPRs() returned %d PRs, want 2", len(filtered))
	}

	// Check that PR 100 (completely released) is not in filtered list
	for _, pr := range filtered {
		if pr.Number == 100 {
			t.Error("filterNonReleasedPRs() should not include completely released PR 100")
		}
	}

	// Check that PR 200 and 300 are in the filtered list
	foundPR200 := false
	foundPR300 := false
	for _, pr := range filtered {
		if pr.Number == 200 {
			foundPR200 = true
		}
		if pr.Number == 300 {
			foundPR300 = true
		}
	}
	if !foundPR200 {
		t.Error("filterNonReleasedPRs() should include PR 200")
	}
	if !foundPR300 {
		t.Error("filterNonReleasedPRs() should include PR 300")
	}
}
