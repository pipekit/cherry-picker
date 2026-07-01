package depmerger

import (
	"fmt"
	"io"
	"sort"

	"github.com/alan/cherry-picker/internal/types"
)

// RenderStatus writes the dependency-PR section of the status output to w.
// execPath and configFlag are used to render suggested next-step commands
// (e.g. "cherry-picker approve 123"); the caller computes them once so the
// cherry-pick and dependency sections stay consistent.
func RenderStatus(w io.Writer, config *Config, execPath, configFlag string, showMerged bool) {
	prsToDisplay := config.TrackedPRs
	if !showMerged {
		prsToDisplay = filterNonMerged(config.TrackedPRs)
	}

	fmt.Fprintf(w, "Dependency PR status for %s/%s\n\n", config.Org, config.Repo)

	if len(prsToDisplay) == 0 {
		if len(config.TrackedPRs) == 0 {
			fmt.Fprintln(w, "No dependency PRs tracked.")
		} else {
			fmt.Fprintln(w, "No active dependency PRs. All PRs are merged (use --show-merged to see them).")
		}
		return
	}

	sort.Slice(prsToDisplay, func(i, j int) bool {
		return prsToDisplay[i].Number < prsToDisplay[j].Number
	})

	for _, pr := range prsToDisplay {
		displayPRStatus(w, pr, config, execPath, configFlag)
	}

	displaySummary(w, prsToDisplay)
}

func filterNonMerged(prs []TrackedPR) []TrackedPR {
	var filtered []TrackedPR
	for _, pr := range prs {
		if !pr.Merged {
			filtered = append(filtered, pr)
		}
	}
	return filtered
}

func displayPRStatus(w io.Writer, pr TrackedPR, config *Config, execPath, configFlag string) {
	url := fmt.Sprintf("https://github.com/%s/%s/pull/%d", config.Org, config.Repo, pr.Number)

	if pr.Title != "" {
		fmt.Fprintf(w, "%s (%s)\n", pr.Title, url)
	} else {
		fmt.Fprintf(w, "%s\n", url)
	}

	if pr.Merged {
		fmt.Fprintf(w, "  Status: ✅ merged\n")
	} else {
		switch pr.CIStatus {
		case CIStatusPassing:
			if pr.Approved {
				fmt.Fprintf(w, "  Status: ✅ CI passing, approved\n")
				fmt.Fprintf(w, "  💡 %s%s merge %d\n", execPath, configFlag, pr.Number)
			} else {
				fmt.Fprintf(w, "  Status: ✅ CI passing\n")
				fmt.Fprintf(w, "  💡 %s%s approve %d\n", execPath, configFlag, pr.Number)
			}
		case CIStatusFailing:
			fmt.Fprintf(w, "  Status: ❌ CI failing")
			if pr.RunAttempt > 0 {
				fmt.Fprintf(w, " [run attempt %d]", pr.RunAttempt)
			}
			fmt.Fprintln(w)
			if len(pr.FailingChecks) > 0 {
				fmt.Fprintf(w, "  Failed: %s\n", types.FormatFailingChecks(pr.FailingChecks))
			}
			fmt.Fprintf(w, "  💡 %s%s retry %d\n", execPath, configFlag, pr.Number)
		case CIStatusPending:
			fmt.Fprintf(w, "  Status: 🔄 CI pending\n")
		case CIStatusUnknown:
			fmt.Fprintf(w, "  Status: ❓ CI unknown\n")
		}
	}
	fmt.Fprintln(w)
}

func displaySummary(w io.Writer, prs []TrackedPR) {
	passing := 0
	approved := 0
	failing := 0
	pending := 0
	merged := 0

	for _, pr := range prs {
		if pr.Merged {
			merged++
			continue
		}
		switch pr.CIStatus {
		case CIStatusPassing:
			passing++
			if pr.Approved {
				approved++
			}
		case CIStatusFailing:
			failing++
		case CIStatusPending:
			pending++
		case CIStatusUnknown:
			// Don't count unknown status PRs in any category
		}
	}

	if approved > 0 {
		fmt.Fprintf(w, "Summary: %d dependency PR(s) - %d passing (%d approved), %d failing, %d pending, %d merged\n",
			len(prs), passing, approved, failing, pending, merged)
	} else {
		fmt.Fprintf(w, "Summary: %d dependency PR(s) - %d passing, %d failing, %d pending, %d merged\n",
			len(prs), passing, failing, pending, merged)
	}
}
