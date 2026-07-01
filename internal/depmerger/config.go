// Package depmerger contains the dependency-PR tracking logic (formerly the
// standalone dep-merger tool): fetching open type/dependencies PRs and
// retrying / merging / approving them. All operations take an injected
// *github.Client and mutate a *Config in memory; persistence is owned by the
// caller (the unified CLI commands and the daemon) via internal/state.
package depmerger

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

// Config is the dependency subsystem's in-memory view. It maps to the
// `dependencies` section of the unified state file (see internal/state); the
// shared Org/Repo/LastFetchDate are populated from the top-level config when a
// view is projected.
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
	Approved      bool     `yaml:"approved,omitempty"`
	Merged        bool     `yaml:"merged"`
}

// FindTrackedPR finds a tracked PR by number, or nil if not tracked.
func FindTrackedPR(config *Config, number int) *TrackedPR {
	for i := range config.TrackedPRs {
		if config.TrackedPRs[i].Number == number {
			return &config.TrackedPRs[i]
		}
	}
	return nil
}
