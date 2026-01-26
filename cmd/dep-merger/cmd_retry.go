package main

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/spf13/cobra"
)

func newRetryCmd(globalConfigFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "retry [pr-number]",
		Short: "Retry failed CI for dependency PRs",
		Long: `Retry failed CI actions for dependency PRs.

If a PR number is provided, retry only that PR.
If no PR number is provided, retry all PRs with failing CI.

Requires GITHUB_TOKEN environment variable to be set.

Examples:
  dep-merger retry           # Retry all PRs with failing CI
  dep-merger retry 123       # Retry PR #123`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := LoadConfig(*globalConfigFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w (run 'dep-merger config' first)", err)
			}

			var prNumber int
			if len(args) > 0 {
				prNumber, err = strconv.Atoi(args[0])
				if err != nil {
					return fmt.Errorf("invalid PR number: %s", args[0])
				}
			}

			return runRetry(cmd.Context(), *globalConfigFile, config, prNumber)
		},
	}
}

func runRetry(ctx context.Context, _ string, config *Config, prNumber int) error {
	client, err := initGitHubClient(ctx, config)
	if err != nil {
		return err
	}

	if prNumber != 0 {
		// Retry specific PR
		pr := findTrackedPR(config, prNumber)
		if pr == nil {
			return fmt.Errorf("PR #%d is not tracked (run 'dep-merger fetch' first)", prNumber)
		}
		if pr.Merged {
			return fmt.Errorf("PR #%d is already merged", prNumber)
		}
		if pr.CIStatus != CIStatusFailing {
			return fmt.Errorf("PR #%d does not have failing CI (status: %s)", prNumber, pr.CIStatus)
		}

		return retrySinglePR(ctx, client, pr)
	}

	// Retry all PRs with failing CI
	var retried int
	for i := range config.TrackedPRs {
		pr := &config.TrackedPRs[i]
		if pr.Merged || pr.CIStatus != CIStatusFailing {
			continue
		}

		if err := retrySinglePR(ctx, client, pr); err != nil {
			slog.Error("Failed to retry PR", "pr", pr.Number, "error", err)
			continue
		}
		retried++
	}

	if retried == 0 {
		fmt.Println("No PRs with failing CI found to retry.")
		return nil
	}

	fmt.Printf("Retried CI for %d PR(s)\n", retried)
	return nil
}

func retrySinglePR(ctx context.Context, client interface {
	RetryFailedWorkflows(context.Context, int) error
}, pr *TrackedPR) error {
	slog.Info("Retrying failed CI for PR", "pr", pr.Number)

	if err := client.RetryFailedWorkflows(ctx, pr.Number); err != nil {
		return fmt.Errorf("failed to retry CI for PR #%d: %w", pr.Number, err)
	}

	fmt.Printf("Successfully triggered retry for PR #%d\n", pr.Number)
	return nil
}
