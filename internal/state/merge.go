package state

import (
	"maps"
	"time"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/depmerger"
)

// The merge functions below reconcile an incoming set of PR data (either a
// full fetch snapshot from the daemon, or a single-subsystem view mutated by a
// CLI command) onto the freshly-reloaded on-disk state. They are monotonic:
// terminal user actions (a merged branch, an approved dep PR) are never
// regressed by a stale-but-concurrent writer. CI freshness follows rank — the
// incoming data wins when its status rank is at least the current one, so the
// daemon keeps refreshing CI while never downgrading an advanced PR.
//
// Deletions are handled asymmetrically. A fetch snapshot (MergeFetched) is a
// full GitHub scrape, so a pending/failed branch absent from it means the
// cherry-pick label was removed upstream — those are deleted, and PRs left
// with no branches are dropped. Branches at picked or beyond are never
// deleted, so a stale snapshot cannot erase a user action that landed
// mid-tick. Command views (MergeCherryView) never remove entries, so their
// merge stays purely additive and a concurrent daemon write survives a
// long-running command's save.

func branchRank(s cmd.BranchStatusType) int {
	switch s {
	case cmd.BranchStatusPending:
		return 0
	case cmd.BranchStatusFailed:
		return 1
	case cmd.BranchStatusPicked:
		return 2
	case cmd.BranchStatusMerged:
		return 3
	case cmd.BranchStatusReleased:
		return 4
	default:
		return 0
	}
}

// MergeFetched overlays a full fetch snapshot onto the receiver. The snapshot
// is authoritative for branch membership: pending/failed branches (and PRs)
// absent from it were removed upstream and are deleted.
func (c *Config) MergeFetched(fetched *Config) {
	c.applyShared(fetched.Org, fetched.Repo, fetched.LastFetchDate)
	mergeCherrySection(&c.CherryPicks, fetched.CherryPicks, true)
	mergeDepSection(&c.Dependencies, fetched.Dependencies)
}

// MergeCherryView overlays a mutated cherry-pick view onto the receiver.
func (c *Config) MergeCherryView(v *cmd.Config) {
	c.applyShared(v.Org, v.Repo, v.LastFetchDate)
	mergeCherrySection(&c.CherryPicks, CherryPickSection{
		SourceBranch:       v.SourceBranch,
		AIAssistantCommand: v.AIAssistantCommand,
		LastCheckedRelease: v.LastCheckedRelease,
		TrackerIssues:      v.TrackerIssues,
		TrackedPRs:         v.TrackedPRs,
	}, false)
}

// MergeDepView overlays a mutated dependency view onto the receiver.
func (c *Config) MergeDepView(v *depmerger.Config) {
	c.applyShared(v.Org, v.Repo, v.LastFetchDate)
	mergeDepSection(&c.Dependencies, DependencySection{TrackedPRs: v.TrackedPRs})
}

func (c *Config) applyShared(org, repo string, date *time.Time) {
	if org != "" {
		c.Org = org
	}
	if repo != "" {
		c.Repo = repo
	}
	if date != nil {
		c.LastFetchDate = date
	}
}

func mergeCherrySection(cur *CherryPickSection, in CherryPickSection, authoritative bool) {
	if in.SourceBranch != "" {
		cur.SourceBranch = in.SourceBranch
	}
	if in.AIAssistantCommand != "" {
		cur.AIAssistantCommand = in.AIAssistantCommand
	}
	cur.LastCheckedRelease = mergeStringMap(cur.LastCheckedRelease, in.LastCheckedRelease)
	cur.TrackerIssues = mergeIntMap(cur.TrackerIssues, in.TrackerIssues)
	cur.TrackedPRs = mergeCherryTracked(cur.TrackedPRs, in.TrackedPRs, authoritative)
}

func mergeCherryTracked(cur, in []cmd.TrackedPR, authoritative bool) []cmd.TrackedPR {
	index := make(map[int]int, len(cur))
	for i := range cur {
		index[cur[i].Number] = i
	}

	inByNumber := make(map[int]*cmd.TrackedPR, len(in))
	for i := range in {
		inByNumber[in[i].Number] = &in[i]
	}

	for _, inPR := range in {
		i, ok := index[inPR.Number]
		if !ok {
			cur = append(cur, inPR)
			index[inPR.Number] = len(cur) - 1
			continue
		}
		curPR := &cur[i]
		if inPR.Title != "" {
			curPR.Title = inPR.Title
		}
		if curPR.Branches == nil && len(inPR.Branches) > 0 {
			curPR.Branches = make(map[string]cmd.BranchStatus, len(inPR.Branches))
		}
		for name, inBranch := range inPR.Branches {
			curBranch, exists := curPR.Branches[name]
			// Take the incoming branch when it is at least as advanced as the
			// current one; keep the current (more advanced) one otherwise.
			if !exists || branchRank(inBranch.Status) >= branchRank(curBranch.Status) {
				curPR.Branches[name] = inBranch
			}
		}
	}

	if !authoritative {
		return cur
	}

	// The incoming snapshot is a full scrape: a pending/failed branch it does
	// not carry had its cherry-pick label removed upstream. Delete those, and
	// drop PRs the snapshot no longer tracks once no branches remain. Branches
	// at picked or beyond are kept regardless, so a stale snapshot can never
	// erase an advanced state.
	kept := cur[:0]
	for i := range cur {
		curPR := &cur[i]
		inPR, tracked := inByNumber[curPR.Number]
		for name, branch := range curPR.Branches {
			if branchRank(branch.Status) > branchRank(cmd.BranchStatusFailed) {
				continue
			}
			if tracked {
				if _, inHasBranch := inPR.Branches[name]; inHasBranch {
					continue
				}
			}
			delete(curPR.Branches, name)
		}
		if !tracked && len(curPR.Branches) == 0 {
			continue
		}
		kept = append(kept, *curPR)
	}
	return kept
}

func mergeDepSection(cur *DependencySection, in DependencySection) {
	cur.TrackedPRs = mergeDepTracked(cur.TrackedPRs, in.TrackedPRs)
}

func mergeDepTracked(cur, in []depmerger.TrackedPR) []depmerger.TrackedPR {
	index := make(map[int]int, len(cur))
	for i := range cur {
		index[cur[i].Number] = i
	}

	for _, inPR := range in {
		i, ok := index[inPR.Number]
		if !ok {
			cur = append(cur, inPR)
			index[inPR.Number] = len(cur) - 1
			continue
		}
		curPR := &cur[i]
		// Terminal flags are monotonic: a user action must never be reverted.
		curPR.Merged = curPR.Merged || inPR.Merged
		curPR.Approved = curPR.Approved || inPR.Approved
		// CI fields track GitHub truth; take the incoming values.
		curPR.Title = inPR.Title
		curPR.CIStatus = inPR.CIStatus
		curPR.RunAttempt = inPR.RunAttempt
		curPR.FailingChecks = inPR.FailingChecks
	}
	return cur
}

func mergeStringMap(cur, in map[string]string) map[string]string {
	if len(in) == 0 {
		return cur
	}
	if cur == nil {
		cur = make(map[string]string, len(in))
	}
	maps.Copy(cur, in)
	return cur
}

func mergeIntMap(cur, in map[string]int) map[string]int {
	if len(in) == 0 {
		return cur
	}
	if cur == nil {
		cur = make(map[string]int, len(in))
	}
	maps.Copy(cur, in)
	return cur
}
