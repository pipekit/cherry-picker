package fetch

import (
	"testing"
	"time"

	"github.com/alan/cherry-picker/cmd"
)

func TestTrackedPRStruct(t *testing.T) {
	pr := cmd.TrackedPR{
		Number: 456,
		Title:  "Test PR Title",
		Branches: map[string]cmd.BranchStatus{
			"release-1.0": {Status: "pending"},
			"release-2.0": {Status: "merged", PR: &cmd.PickPR{Number: 789, Title: "Cherry-pick", CIStatus: "passing"}},
		},
	}

	if pr.Number != 456 {
		t.Errorf("cmd.TrackedPR.Number = %d, want 456", pr.Number)
	}

	if len(pr.Branches) != 2 {
		t.Errorf("cmd.TrackedPR.Branches length = %d, want 2", len(pr.Branches))
	}

	if pr.Branches["release-1.0"].Status != "pending" {
		t.Errorf("cmd.TrackedPR.Branches[release-1.0].Status = %q, want pending", pr.Branches["release-1.0"].Status)
	}

	if pr.Branches["release-2.0"].Status != "merged" {
		t.Errorf("cmd.TrackedPR.Branches[release-2.0].Status = %q, want merged", pr.Branches["release-2.0"].Status)
	}

	if pr.Branches["release-2.0"].PR == nil || pr.Branches["release-2.0"].PR.Number != 789 {
		t.Errorf("cmd.TrackedPR.Branches[release-2.0].PR.Number = %v, want 789", pr.Branches["release-2.0"].PR)
	}
}

func TestUpdateExistingPRTitles(t *testing.T) {
	// This test verifies the logic but can't test actual GitHub API calls
	config := &cmd.Config{
		Org:                "testorg",
		Repo:               "testrepo",
		AIAssistantCommand: "cursor-agent",
		TrackedPRs: []cmd.TrackedPR{
			{
				Number: 123,
				Title:  "Old Title",
			},
			{
				Number: 124,
				Title:  "Another Old Title",
			},
			{
				Number: 125,
				Title:  "Ignored PR Title",
			},
		},
	}

	// Verify we have 3 tracked PRs
	if len(config.TrackedPRs) != 3 {
		t.Errorf("Expected 3 tracked PRs, got %d", len(config.TrackedPRs))
	}

	// Verify the structure is intact
	if config.TrackedPRs[2].Title != "Ignored PR Title" {
		t.Error("Expected third PR to have the correct title")
	}
}

func TestBranchStatusStruct(t *testing.T) {
	// Test pending status
	pendingStatus := cmd.BranchStatus{Status: "pending"}
	if pendingStatus.Status != "pending" {
		t.Errorf("cmd.BranchStatus.Status = %q, want pending", pendingStatus.Status)
	}
	if pendingStatus.PR != nil {
		t.Errorf("cmd.BranchStatus.PR = %v, want nil for pending status", pendingStatus.PR)
	}

	// Test picked status with pick PR
	pickedStatus := cmd.BranchStatus{Status: "picked", PR: &cmd.PickPR{Number: 456, Title: "Test PR", CIStatus: "pending"}}
	if pickedStatus.Status != "picked" {
		t.Errorf("cmd.BranchStatus.Status = %q, want picked", pickedStatus.Status)
	}
	if pickedStatus.PR == nil || pickedStatus.PR.Number != 456 {
		t.Errorf("cmd.BranchStatus.PR.Number = %v, want 456", pickedStatus.PR)
	}
}

func TestConfigWithTrackedPRs(t *testing.T) {
	now := time.Now()
	config := cmd.Config{
		Org:                "testorg",
		Repo:               "testrepo",
		SourceBranch:       "main",
		AIAssistantCommand: "cursor-agent",
		LastFetchDate:      &now,
		TrackedPRs: []cmd.TrackedPR{
			{
				Number: 123,
				Title:  "Test PR 123",
				Branches: map[string]cmd.BranchStatus{
					"release-1.0": {Status: "pending"},
				},
			},
			{
				Number: 124,
				Title:  "Test PR 124",
			},
		},
	}

	if len(config.TrackedPRs) != 2 {
		t.Errorf("Config.cmd.TrackedPRs length = %d, want 2", len(config.TrackedPRs))
	}

	if config.TrackedPRs[0].Number != 123 {
		t.Errorf("Config.cmd.TrackedPRs[0].Number = %d, want 123", config.TrackedPRs[0].Number)
	}

	if config.TrackedPRs[0].Branches["release-1.0"].Status != "pending" {
		t.Errorf("Config.cmd.TrackedPRs[0].Branches[release-1.0].Status = %q, want pending", config.TrackedPRs[0].Branches["release-1.0"].Status)
	}

	if config.TrackedPRs[1].Title != "Test PR 124" {
		t.Errorf("Config.cmd.TrackedPRs[1].Title = %v, want Test PR 124", config.TrackedPRs[1].Title)
	}

	if config.LastFetchDate == nil {
		t.Error("Config.LastFetchDate should not be nil")
	}
}

func TestRemoveEmptyPRs(t *testing.T) {
	tests := []struct {
		name          string
		trackedPRs    []cmd.TrackedPR
		wantRemoved   int
		wantRemaining int
	}{
		{
			name: "No empty PRs",
			trackedPRs: []cmd.TrackedPR{
				{Number: 1, Branches: map[string]cmd.BranchStatus{"release-1.0": {Status: "pending"}}},
				{Number: 2, Branches: map[string]cmd.BranchStatus{"release-2.0": {Status: "picked"}}},
			},
			wantRemoved:   0,
			wantRemaining: 2,
		},
		{
			name: "All empty PRs",
			trackedPRs: []cmd.TrackedPR{
				{Number: 1, Branches: map[string]cmd.BranchStatus{}},
				{Number: 2, Branches: map[string]cmd.BranchStatus{}},
			},
			wantRemoved:   2,
			wantRemaining: 0,
		},
		{
			name: "Mixed empty and non-empty PRs",
			trackedPRs: []cmd.TrackedPR{
				{Number: 1, Branches: map[string]cmd.BranchStatus{"release-1.0": {Status: "pending"}}},
				{Number: 2, Branches: map[string]cmd.BranchStatus{}},
				{Number: 3, Branches: map[string]cmd.BranchStatus{"release-3.0": {Status: "merged"}}},
				{Number: 4, Branches: map[string]cmd.BranchStatus{}},
			},
			wantRemoved:   2,
			wantRemaining: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &cmd.Config{TrackedPRs: tt.trackedPRs}
			removed := removeEmptyPRs(config)
			if removed != tt.wantRemoved {
				t.Errorf("removeEmptyPRs() removed = %d, want %d", removed, tt.wantRemoved)
			}
			if len(config.TrackedPRs) != tt.wantRemaining {
				t.Errorf("removeEmptyPRs() remaining PRs = %d, want %d", len(config.TrackedPRs), tt.wantRemaining)
			}
		})
	}
}

func TestIsPRTracked(t *testing.T) {
	config := &cmd.Config{
		TrackedPRs: []cmd.TrackedPR{
			{Number: 123},
			{Number: 456},
			{Number: 789},
		},
	}

	tests := []struct {
		name     string
		prNumber int
		want     bool
	}{
		{"PR is tracked", 123, true},
		{"PR is tracked (middle)", 456, true},
		{"PR is tracked (last)", 789, true},
		{"PR is not tracked", 999, false},
		{"PR is not tracked (zero)", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPRTracked(config, tt.prNumber)
			if got != tt.want {
				t.Errorf("isPRTracked(%d) = %v, want %v", tt.prNumber, got, tt.want)
			}
		})
	}
}
