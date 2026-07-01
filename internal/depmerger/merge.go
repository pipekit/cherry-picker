package depmerger

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alan/cherry-picker/internal/github"
)

// MergePRs squash-merges dependency PRs with passing CI. If prNumber is
// non-zero only that PR is merged; otherwise all eligible PRs are merged.
// config is mutated in place (Merged flags); the caller persists.
func MergePRs(ctx context.Context, client *github.Client, config *Config, prNumber int) error {
	if prNumber != 0 {
		pr, err := validatePRForOperation(config, prNumber, CIStatusPassing, "merge")
		if err != nil {
			return err
		}
		return mergeSinglePR(ctx, client, pr)
	}

	merged := executeBulkPROperation(ctx, config, CIStatusPassing, func(ctx context.Context, pr *TrackedPR) error {
		return mergeSinglePR(ctx, client, pr)
	}, "merge")

	if merged == 0 {
		fmt.Println("No dependency PRs with passing CI found to merge.")
		return nil
	}

	fmt.Printf("Merged %d dependency PR(s)\n", merged)
	return nil
}

func mergeSinglePR(ctx context.Context, client *github.Client, pr *TrackedPR) error {
	slog.Info("Merging PR", "pr", pr.Number)

	if err := client.MergePR(ctx, pr.Number, "squash"); err != nil {
		return fmt.Errorf("failed to merge PR #%d: %w", pr.Number, err)
	}

	pr.Merged = true
	fmt.Printf("Successfully merged PR #%d\n", pr.Number)
	return nil
}
