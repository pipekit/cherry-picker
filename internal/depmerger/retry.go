package depmerger

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alan/cherry-picker/internal/github"
)

// RetryPRs retries failed CI for dependency PRs. If prNumber is non-zero only
// that PR is retried; otherwise all PRs with failing CI are retried. config is
// not mutated (a retry does not change tracked state).
func RetryPRs(ctx context.Context, client *github.Client, config *Config, prNumber int) error {
	if prNumber != 0 {
		pr, err := validatePRForOperation(config, prNumber, CIStatusFailing, "retry")
		if err != nil {
			return err
		}
		return retrySinglePR(ctx, client, pr)
	}

	retried := executeBulkPROperation(ctx, config, CIStatusFailing, func(ctx context.Context, pr *TrackedPR) error {
		return retrySinglePR(ctx, client, pr)
	}, "retry")

	if retried == 0 {
		fmt.Println("No dependency PRs with failing CI found to retry.")
		return nil
	}

	fmt.Printf("Retried CI for %d dependency PR(s)\n", retried)
	return nil
}

func retrySinglePR(ctx context.Context, client *github.Client, pr *TrackedPR) error {
	slog.Info("Retrying failed CI for PR", "pr", pr.Number)

	if err := client.RetryFailedWorkflows(ctx, pr.Number); err != nil {
		return fmt.Errorf("failed to retry CI for PR #%d: %w", pr.Number, err)
	}

	fmt.Printf("Successfully triggered retry for PR #%d\n", pr.Number)
	return nil
}
