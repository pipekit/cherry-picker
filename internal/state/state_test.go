package state

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/depmerger"
	"github.com/alan/cherry-picker/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tmpConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "cherry-picker.yaml")
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := tmpConfigPath(t)
	now := time.Now().UTC().Truncate(time.Second)
	in := &Config{
		Org:           "acme",
		Repo:          "widget",
		LastFetchDate: &now,
		CherryPicks: CherryPickSection{
			SourceBranch:       "main",
			AIAssistantCommand: "claude",
			LastCheckedRelease: map[string]string{"release-3.6": "v3.6.1"},
			TrackerIssues:      map[string]int{"release-3.6": 42},
			TrackedPRs: []cmd.TrackedPR{{
				Number: 100, Title: "fix",
				Branches: map[string]cmd.BranchStatus{
					"release-3.6": {Status: cmd.BranchStatusMerged, PR: &cmd.PickPR{Number: 200, CIStatus: types.CIStatusPassing}},
				},
			}},
		},
		Dependencies: DependencySection{
			TrackedPRs: []depmerger.TrackedPR{{Number: 300, Title: "bump", CIStatus: types.CIStatusPassing, Approved: true}},
		},
	}

	require.NoError(t, Save(path, in))
	out, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestUpdateReloadMerge(t *testing.T) {
	path := tmpConfigPath(t)
	require.NoError(t, Save(path, &Config{Org: "acme", Repo: "widget"}))

	err := Update(path, func(c *Config) error {
		c.Dependencies.TrackedPRs = append(c.Dependencies.TrackedPRs, depmerger.TrackedPR{Number: 1, Merged: true})
		return nil
	})
	require.NoError(t, err)

	out, err := Load(path)
	require.NoError(t, err)
	require.Len(t, out.Dependencies.TrackedPRs, 1)
	assert.True(t, out.Dependencies.TrackedPRs[0].Merged)
	assert.Equal(t, "acme", out.Org)
}

func TestMergeFetchedDoesNotRegressCherryBranch(t *testing.T) {
	// User advanced the branch to merged; a stale fetch snapshot still shows picked.
	cur := &Config{CherryPicks: CherryPickSection{TrackedPRs: []cmd.TrackedPR{{
		Number:   1,
		Branches: map[string]cmd.BranchStatus{"release-3.6": {Status: cmd.BranchStatusMerged}},
	}}}}
	fetched := &Config{CherryPicks: CherryPickSection{TrackedPRs: []cmd.TrackedPR{{
		Number:   1,
		Branches: map[string]cmd.BranchStatus{"release-3.6": {Status: cmd.BranchStatusPicked}},
	}}}}

	cur.MergeFetched(fetched)
	assert.Equal(t, cmd.BranchStatusMerged, cur.CherryPicks.TrackedPRs[0].Branches["release-3.6"].Status)
}

func TestMergeFetchedAdvancesCherryBranch(t *testing.T) {
	// Fetch sees the branch advance from picked to merged; it should apply.
	cur := &Config{CherryPicks: CherryPickSection{TrackedPRs: []cmd.TrackedPR{{
		Number:   1,
		Branches: map[string]cmd.BranchStatus{"release-3.6": {Status: cmd.BranchStatusPicked}},
	}}}}
	fetched := &Config{CherryPicks: CherryPickSection{TrackedPRs: []cmd.TrackedPR{{
		Number:   1,
		Branches: map[string]cmd.BranchStatus{"release-3.6": {Status: cmd.BranchStatusMerged}},
	}}}}

	cur.MergeFetched(fetched)
	assert.Equal(t, cmd.BranchStatusMerged, cur.CherryPicks.TrackedPRs[0].Branches["release-3.6"].Status)
}

func TestMergeDepMonotonicFlagsAndFreshCI(t *testing.T) {
	// User approved+merged PR 1; stale fetch shows neither, but fresher CI.
	cur := &Config{Dependencies: DependencySection{TrackedPRs: []depmerger.TrackedPR{
		{Number: 1, Approved: true, Merged: true, CIStatus: types.CIStatusPending},
	}}}
	fetched := &Config{Dependencies: DependencySection{TrackedPRs: []depmerger.TrackedPR{
		{Number: 1, Approved: false, Merged: false, CIStatus: types.CIStatusPassing},
		{Number: 2, CIStatus: types.CIStatusFailing},
	}}}

	cur.MergeFetched(fetched)
	require.Len(t, cur.Dependencies.TrackedPRs, 2)
	assert.True(t, cur.Dependencies.TrackedPRs[0].Approved, "approval must not be reverted")
	assert.True(t, cur.Dependencies.TrackedPRs[0].Merged, "merged must not be reverted")
	assert.Equal(t, types.CIStatusPassing, cur.Dependencies.TrackedPRs[0].CIStatus, "CI should refresh")
	assert.Equal(t, 2, cur.Dependencies.TrackedPRs[1].Number, "new PR added")
}

func TestViewsRoundTrip(t *testing.T) {
	c := &Config{Org: "acme", Repo: "widget", CherryPicks: CherryPickSection{SourceBranch: "main"}}
	cv := c.CherryView()
	assert.Equal(t, "acme", cv.Org)
	assert.Equal(t, "main", cv.SourceBranch)
	dv := c.DepView()
	assert.Equal(t, "widget", dv.Repo)
}
