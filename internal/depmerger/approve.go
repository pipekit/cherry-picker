package depmerger

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alan/cherry-picker/internal/github"
)

// ApprovePRs approves dependency PRs. If prNumber is non-zero only that PR is
// approved (CI status is not required); otherwise all not-yet-approved PRs with
// passing CI are approved. config is mutated in place (Approved flags); the
// caller persists.
func ApprovePRs(ctx context.Context, client *github.Client, config *Config, prNumber int) error {
	if prNumber != 0 {
		pr, err := validateTrackedPR(config, prNumber)
		if err != nil {
			return err
		}
		if err := validatePRNotMerged(pr); err != nil {
			return err
		}
		if pr.Approved {
			return fmt.Errorf("PR #%d is already approved", prNumber)
		}

		if err := approveSinglePR(ctx, client, pr); err != nil {
			return err
		}
		pr.Approved = true
		return nil
	}

	approved := executeBulkPROperation(ctx, config, CIStatusPassing, func(ctx context.Context, pr *TrackedPR) error {
		if pr.Approved {
			return nil // Skip already approved
		}
		if err := approveSinglePR(ctx, client, pr); err != nil {
			return err
		}
		pr.Approved = true
		return nil
	}, "approve")

	if approved == 0 {
		fmt.Println("No dependency PRs with passing CI found to approve.")
		return nil
	}

	fmt.Printf("Approved %d dependency PR(s)\n", approved)
	return nil
}

func approveSinglePR(ctx context.Context, client *github.Client, pr *TrackedPR) error {
	slog.Info("Approving PR", "pr", pr.Number)

	if err := client.ApprovePR(ctx, pr.Number); err != nil {
		return fmt.Errorf("failed to approve PR #%d: %w", pr.Number, err)
	}

	fmt.Printf("Successfully approved PR #%d\n", pr.Number)
	return nil
}
