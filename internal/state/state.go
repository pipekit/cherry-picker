// Package state owns the single unified on-disk config+state file for the
// merged cherry-picker tool. The file holds shared org/repo/last_fetch_date
// plus a cherry_picks section and a dependencies section. The two subsystems
// keep their existing in-memory types (cmd.Config / depmerger.Config); this
// package projects to and from them via views, so command code is unchanged.
//
// Writers serialize through Update (flock + reload + atomic save); readers call
// Load without locking and rely on the atomic rename in Save.
package state

import (
	"time"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/depmerger"
)

// Config is the unified on-disk representation.
type Config struct {
	Org           string            `yaml:"org"`
	Repo          string            `yaml:"repo"`
	LastFetchDate *time.Time        `yaml:"last_fetch_date,omitempty"`
	CherryPicks   CherryPickSection `yaml:"cherry_picks"`
	Dependencies  DependencySection `yaml:"dependencies"`
}

// CherryPickSection holds the cherry-pick subsystem's config and tracked PRs.
type CherryPickSection struct {
	SourceBranch       string            `yaml:"source_branch"`
	AIAssistantCommand string            `yaml:"ai_assistant_command"`
	LastCheckedRelease map[string]string `yaml:"last_checked_release,omitempty"` // branch -> last checked release tag
	TrackerIssues      map[string]int    `yaml:"tracker_issues,omitempty"`       // branch -> tracker issue number
	TrackedPRs         []cmd.TrackedPR   `yaml:"tracked_prs,omitempty"`
}

// DependencySection holds the dependency subsystem's tracked PRs.
type DependencySection struct {
	TrackedPRs []depmerger.TrackedPR `yaml:"tracked_prs,omitempty"`
}

// CherryView projects the shared fields plus the cherry-pick section into the
// cmd.Config type the cherry-pick commands operate on. Slices and maps are
// shared with the receiver; mutations should be written back via
// ApplyCherryView / MergeCherryView.
func (c *Config) CherryView() *cmd.Config {
	return &cmd.Config{
		Org:                c.Org,
		Repo:               c.Repo,
		SourceBranch:       c.CherryPicks.SourceBranch,
		AIAssistantCommand: c.CherryPicks.AIAssistantCommand,
		LastFetchDate:      c.LastFetchDate,
		LastCheckedRelease: c.CherryPicks.LastCheckedRelease,
		TrackerIssues:      c.CherryPicks.TrackerIssues,
		TrackedPRs:         c.CherryPicks.TrackedPRs,
	}
}

// ApplyCherryView overwrites the shared fields and cherry-pick section from a
// (possibly mutated) view. Use when there is no concurrent writer to reconcile
// with; otherwise prefer MergeCherryView.
func (c *Config) ApplyCherryView(v *cmd.Config) {
	c.Org = v.Org
	c.Repo = v.Repo
	c.LastFetchDate = v.LastFetchDate
	c.CherryPicks.SourceBranch = v.SourceBranch
	c.CherryPicks.AIAssistantCommand = v.AIAssistantCommand
	c.CherryPicks.LastCheckedRelease = v.LastCheckedRelease
	c.CherryPicks.TrackerIssues = v.TrackerIssues
	c.CherryPicks.TrackedPRs = v.TrackedPRs
}

// DepView projects the shared fields plus the dependency section into the
// depmerger.Config type the dependency commands operate on.
func (c *Config) DepView() *depmerger.Config {
	return &depmerger.Config{
		Org:           c.Org,
		Repo:          c.Repo,
		LastFetchDate: c.LastFetchDate,
		TrackedPRs:    c.Dependencies.TrackedPRs,
	}
}

// ApplyDepView overwrites the shared fields and dependency section from a
// (possibly mutated) view.
func (c *Config) ApplyDepView(v *depmerger.Config) {
	c.Org = v.Org
	c.Repo = v.Repo
	c.LastFetchDate = v.LastFetchDate
	c.Dependencies.TrackedPRs = v.TrackedPRs
}
