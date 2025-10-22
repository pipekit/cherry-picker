// Package cmd defines core data structures for cherry-picker configuration and PR tracking.
package cmd

import "time"

// CIStatus represents the status of CI checks
type CIStatus string

const (
	// CIStatusPassing indicates all CI checks have passed
	CIStatusPassing CIStatus = "passing"
	// CIStatusFailing indicates one or more CI checks have failed
	CIStatusFailing CIStatus = "failing"
	// CIStatusPending indicates CI checks are still running
	CIStatusPending CIStatus = "pending"
	// CIStatusUnknown indicates CI status could not be determined
	CIStatusUnknown CIStatus = "unknown"
)

// ParseCIStatus converts a string to CIStatus
func ParseCIStatus(s string) CIStatus {
	switch s {
	case "passing":
		return CIStatusPassing
	case "failing":
		return CIStatusFailing
	case "pending":
		return CIStatusPending
	default:
		return CIStatusUnknown
	}
}

// BranchStatusType represents the status of a PR for a specific target branch
type BranchStatusType string

const (
	// BranchStatusPending indicates bot hasn't attempted cherry-pick yet
	BranchStatusPending BranchStatusType = "pending"
	// BranchStatusFailed indicates bot attempted but failed (usually conflicts)
	BranchStatusFailed BranchStatusType = "failed"
	// BranchStatusPicked indicates bot or manual pick successfully created cherry-pick PR
	BranchStatusPicked BranchStatusType = "picked"
	// BranchStatusMerged indicates cherry-pick PR has been merged
	BranchStatusMerged BranchStatusType = "merged"
	// BranchStatusReleased indicates cherry-pick PR has been included in a release
	BranchStatusReleased BranchStatusType = "released"
)

// ParseBranchStatus converts a string to BranchStatusType
func ParseBranchStatus(s string) BranchStatusType {
	switch s {
	case "pending":
		return BranchStatusPending
	case "failed":
		return BranchStatusFailed
	case "picked":
		return BranchStatusPicked
	case "merged":
		return BranchStatusMerged
	case "released":
		return BranchStatusReleased
	default:
		return BranchStatusPending // Default to pending for unknown values
	}
}

// Config represents the structure of cherry-picks.yaml
type Config struct {
	Org                string            `yaml:"org"`
	Repo               string            `yaml:"repo"`
	SourceBranch       string            `yaml:"source_branch"`
	AIAssistantCommand string            `yaml:"ai_assistant_command"`
	LastFetchDate      *time.Time        `yaml:"last_fetch_date,omitempty"`
	LastCheckedRelease map[string]string `yaml:"last_checked_release,omitempty"` // branch -> last checked release tag
	TrackedPRs         []TrackedPR       `yaml:"tracked_prs,omitempty"`
}

// TrackedPR represents a PR that we're tracking for cherry-picking
type TrackedPR struct {
	Number   int                     `yaml:"number"`
	Title    string                  `yaml:"title"`
	Branches map[string]BranchStatus `yaml:"branches,omitempty"`
}

// BranchStatus represents the status of a PR for a specific target branch
type BranchStatus struct {
	Status BranchStatusType `yaml:"status"`
	PR     *PickPR          `yaml:"pr,omitempty"` // Details of the cherry-pick PR (if picked or merged)
}

// PickPR represents the PR that was cherry-picked
type PickPR struct {
	Number   int      `yaml:"number"`
	CIStatus CIStatus `yaml:"ci_status"`
	Title    string   `yaml:"title"`
}
