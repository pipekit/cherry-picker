package main

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/alan/cherry-picker/internal/github"
	"github.com/spf13/cobra"
)

func newMergeCmd(globalConfigFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "merge [pr-number]",
		Short: "Squash and merge dependency PRs with passing CI",
		Long: `Squash and merge dependency PRs that have passing CI.

If a PR number is provided, merge only that PR.
If no PR number is provided, merge all PRs with passing CI.

Requires GITHUB_TOKEN environment variable to be set.

Examples:
  dep-merger merge           # Merge all PRs with passing CI
  dep-merger merge 123       # Merge PR #123`,
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

			return runMerge(cmd.Context(), *globalConfigFile, config, prNumber)
		},
	}
}

func runMerge(ctx context.Context, configFile string, config *Config, prNumber int) error {
	client, err := initGitHubClient(ctx, config)
	if err != nil {
		return err
	}

	if prNumber != 0 {
		// Merge specific PR
		pr := findTrackedPR(config, prNumber)
		if pr == nil {
			return fmt.Errorf("PR #%d is not tracked (run 'dep-merger fetch' first)", prNumber)
		}
		if pr.Merged {
			return fmt.Errorf("PR #%d is already merged", prNumber)
		}
		if pr.CIStatus != CIStatusPassing {
			return fmt.Errorf("PR #%d does not have passing CI (status: %s)", prNumber, pr.CIStatus)
		}

		if err := mergeSinglePR(ctx, client, pr); err != nil {
			return err
		}

		return SaveConfig(configFile, config)
	}

	// Merge all PRs with passing CI
	var merged int
	for i := range config.TrackedPRs {
		pr := &config.TrackedPRs[i]
		if pr.Merged || pr.CIStatus != CIStatusPassing {
			continue
		}

		if err := mergeSinglePR(ctx, client, pr); err != nil {
			slog.Error("Failed to merge PR", "pr", pr.Number, "error", err)
			continue
		}
		merged++
	}

	if merged == 0 {
		fmt.Println("No PRs with passing CI found to merge.")
		return nil
	}

	if err := SaveConfig(configFile, config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Merged %d PR(s)\n", merged)
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
