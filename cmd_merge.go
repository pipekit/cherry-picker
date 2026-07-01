package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/alan/cherry-picker/cmd/merge"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/alan/cherry-picker/internal/depmerger"
	"github.com/alan/cherry-picker/internal/github"
	"github.com/alan/cherry-picker/internal/state"
	"github.com/spf13/cobra"
)

func newMergeCmd(configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "merge [pr-number] [target-branch]",
		Short: "Squash-merge eligible cherry-pick or dependency PRs with passing CI",
		Long: `Squash-merge PRs with passing CI. With no PR number, merges all eligible
PRs in both subsystems. With a PR number, dispatches to whichever subsystem
tracks it (cherry-pick or dependency). A target branch applies only to
cherry-pick PRs.

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
			return dispatchMerge(ctx, client, st, *configFile, prNumber, targetBranch)
		},
	}
}

func dispatchMerge(ctx context.Context, client *github.Client, st *state.Config, configFile string, prNumber int, targetBranch string) error {
	base := commands.BaseCommand{
		ConfigFile:   &configFile,
		LoadConfig:   loadCherry,
		SaveConfig:   saveCherry,
		GitHubClient: client,
		Config:       st.CherryView(),
	}

	if prNumber == 0 {
		var errs []error
		if err := merge.Execute(ctx, base, 0, ""); err != nil {
			errs = append(errs, err)
		}
		if err := runDepMerge(ctx, client, configFile, st.DepView(), 0); err != nil {
			errs = append(errs, err)
		}
		return errors.Join(errs...)
	}

	if prTrackedInCherry(st, prNumber) {
		return merge.Execute(ctx, base, prNumber, targetBranch)
	}
	if depmerger.FindTrackedPR(st.DepView(), prNumber) != nil {
		return runDepMerge(ctx, client, configFile, st.DepView(), prNumber)
	}
	return fmt.Errorf("PR #%d is not tracked in either subsystem (run 'fetch' first)", prNumber)
}

func runDepMerge(ctx context.Context, client *github.Client, configFile string, dv *depmerger.Config, prNumber int) error {
	if err := depmerger.MergePRs(ctx, client, dv, prNumber); err != nil {
		return err
	}
	return saveDep(configFile, dv)
}
