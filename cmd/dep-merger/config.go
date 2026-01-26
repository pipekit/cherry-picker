// Package main defines core data structures for dep-merger configuration and PR tracking.
package main

import (
	"time"

	"github.com/alan/cherry-picker/internal/types"
)

// CIStatus is an alias for types.CIStatus for backward compatibility
type CIStatus = types.CIStatus

// CI status constants from internal/types package
const (
	CIStatusPassing = types.CIStatusPassing
	CIStatusFailing = types.CIStatusFailing
	CIStatusPending = types.CIStatusPending
	CIStatusUnknown = types.CIStatusUnknown
)

// ParseCIStatus is an alias for types.ParseCIStatus
var ParseCIStatus = types.ParseCIStatus

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
