package cmd

import "time"

// CIStatus represents the status of CI checks
type CIStatus string

const (
	CIStatusPassing CIStatus = "passing"
	CIStatusFailing CIStatus = "failing"
	CIStatusPending CIStatus = "pending"
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

// BranchStatus represents the status of a PR for a specific target branch
type BranchStatusType string

const (
	BranchStatusPending BranchStatusType = "pending"
	BranchStatusPicked  BranchStatusType = "picked"
	BranchStatusMerged  BranchStatusType = "merged"
)

// ParseBranchStatus converts a string to BranchStatusType
func ParseBranchStatus(s string) BranchStatusType {
	switch s {
	case "pending":
		return BranchStatusPending
	case "picked":
		return BranchStatusPicked
	case "merged":
		return BranchStatusMerged
	default:
		return BranchStatusPending // Default to pending for unknown values
	}
}

// Config represents the structure of cherry-picks.yaml
type Config struct {
	Org           string      `yaml:"org"`
	Repo          string      `yaml:"repo"`
	SourceBranch  string      `yaml:"source_branch"`
	LastFetchDate *time.Time  `yaml:"last_fetch_date,omitempty"`
	TrackedPRs    []TrackedPR `yaml:"tracked_prs,omitempty"`
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
