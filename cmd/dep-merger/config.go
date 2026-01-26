// Package main defines core data structures for dep-merger configuration and PR tracking.
package main

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

// Config represents the structure of dep-merger.yaml
type Config struct {
	Org           string      `yaml:"org"`
	Repo          string      `yaml:"repo"`
	LastFetchDate *time.Time  `yaml:"last_fetch_date,omitempty"`
	TrackedPRs    []TrackedPR `yaml:"tracked_prs,omitempty"`
}

// TrackedPR represents a dependency PR that we're tracking
type TrackedPR struct {
	Number        int      `yaml:"number"`
	Title         string   `yaml:"title"`
	CIStatus      CIStatus `yaml:"ci_status"`
	RunAttempt    int      `yaml:"run_attempt,omitempty"`
	FailingChecks []string `yaml:"failing_checks,omitempty"` // Names of failing CI checks (only populated when CI is failing)
	Merged        bool     `yaml:"merged"`
}
