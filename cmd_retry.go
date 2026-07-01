package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/alan/cherry-picker/cmd/retry"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/alan/cherry-picker/internal/depmerger"
	"github.com/alan/cherry-picker/internal/github"
	"github.com/alan/cherry-picker/internal/state"
	"github.com/spf13/cobra"
)

func newRetryCmd(configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "retry [pr-number] [target-branch]",
		Short: "Retry failed CI for cherry-pick or dependency PRs",
		Long: `Retry failed CI workflows. With no PR number, retries all PRs with
failing CI in both subsystems. With a PR number, dispatches to whichever
subsystem tracks it. A target branch applies only to cherry-pick PRs.

Requires GITHUB_TOKEN environment variable to be set.`,
		Args:         cobra.RangeArgs(0, 2),
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			ctx := cobraCmd.Context()
			prNumber, err := commands.ParsePRNumberFromArgs(args, false)
			if err != nil {
				return err
			}
			targetBranch := commands.GetTargetBranchFromArgs(args)

			client, st, err := loadStateAndClient(ctx, *configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			return dispatchRetry(ctx, client, st, *configFile, prNumber, targetBranch)
		},
	}
}

func dispatchRetry(ctx context.Context, client *github.Client, st *state.Config, configFile string, prNumber int, targetBranch string) error {
	base := commands.BaseCommand{
		ConfigFile:   &configFile,
		LoadConfig:   loadCherry,
		SaveConfig:   saveCherry,
		GitHubClient: client,
		Config:       st.CherryView(),
	}

	if prNumber == 0 {
		var errs []error
		if err := retry.Execute(ctx, base, 0, ""); err != nil {
			errs = append(errs, err)
		}
		// Retry does not mutate tracked state, so no save is needed.
		if err := depmerger.RetryPRs(ctx, client, st.DepView(), 0); err != nil {
			errs = append(errs, err)
		}
		return errors.Join(errs...)
	}

	if prTrackedInCherry(st, prNumber) {
		return retry.Execute(ctx, base, prNumber, targetBranch)
	}
	if depmerger.FindTrackedPR(st.DepView(), prNumber) != nil {
		return depmerger.RetryPRs(ctx, client, st.DepView(), prNumber)
	}
	return fmt.Errorf("PR #%d is not tracked in either subsystem (run 'fetch' first)", prNumber)
}
