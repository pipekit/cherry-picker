package depmerger

import (
	"context"
	"fmt"

	"github.com/alan/cherry-picker/internal/github"
)

const dependenciesLabel = "type/dependencies"

// RefreshDeps fetches open PRs with the type/dependencies label and updates the
// tracked-PR list in config in place. It performs no file I/O and does not set
// LastFetchDate; the caller owns persistence and the shared timestamp.
func RefreshDeps(ctx context.Context, client *github.Client, config *Config) error {
	fmt.Printf("Fetching open PRs with label '%s' from %s/%s...\n", dependenciesLabel, config.Org, config.Repo)

	prs, err := client.GetOpenPRsWithLabel(ctx, dependenciesLabel)
	if err != nil {
		return fmt.Errorf("failed to fetch dependency PRs: %w", err)
	}

	if len(prs) == 0 {
		fmt.Println("No open dependency PRs found.")
		markClosedPRs(config, prs)
		return nil
	}

	fmt.Printf("Found %d open dependency PR(s)\n", len(prs))

	newCount := 0
	updatedCount := 0

	for _, pr := range prs {
		prDetails, err := client.GetPRWithDetailsNoDCOFilter(ctx, pr.Number)
		if err != nil {
			fmt.Printf("  Warning: failed to get details for PR #%d: %v\n", pr.Number, err)
			continue
		}

		approved, err := client.IsPRApproved(ctx, pr.Number)
		if err != nil {
			fmt.Printf("  Warning: failed to check approval for PR #%d: %v\n", pr.Number, err)
		}

		if existing := FindTrackedPR(config, pr.Number); existing != nil {
			existing.Title = prDetails.Title
			existing.CIStatus = ParseCIStatus(prDetails.CIStatus)
			existing.RunAttempt = prDetails.RunAttempt
			existing.FailingChecks = prDetails.FailingChecks
			existing.Approved = approved
			updatedCount++
		} else {
			config.TrackedPRs = append(config.TrackedPRs, TrackedPR{
				Number:        prDetails.Number,
				Title:         prDetails.Title,
				CIStatus:      ParseCIStatus(prDetails.CIStatus),
				RunAttempt:    prDetails.RunAttempt,
				FailingChecks: prDetails.FailingChecks,
				Approved:      approved,
				Merged:        false,
			})
			newCount++
			fmt.Printf("  Added: #%d %s\n", prDetails.Number, prDetails.Title)
		}
	}

	// Mark PRs that are no longer open (merged or closed externally).
	markClosedPRs(config, prs)

	fmt.Printf("Dependency fetch complete: %d new, %d updated\n", newCount, updatedCount)
	return nil
}

func markClosedPRs(config *Config, openPRs []github.PR) {
	openPRNumbers := make(map[int]bool)
	for _, pr := range openPRs {
		openPRNumbers[pr.Number] = true
	}

	for i := range config.TrackedPRs {
		if !config.TrackedPRs[i].Merged && !openPRNumbers[config.TrackedPRs[i].Number] {
			// PR is no longer open and wasn't merged by us - assume it was merged externally
			config.TrackedPRs[i].Merged = true
		}
	}
}
