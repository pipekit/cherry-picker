// Package merge implements the merge command for squash-merging cherry-pick PRs with passing CI.
package merge

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/alan/cherry-picker/internal/github"
	"github.com/spf13/cobra"
)

// command encapsulates the merge command with common functionality
type command struct {
	commands.BaseCommand
	PRNumber     int
	TargetBranch string
}

// NewMergeCmd creates the merge command
func NewMergeCmd(loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) *cobra.Command {
	mergeCmd := &command{}
	var configFile string

	cobraCmd := &cobra.Command{
		Use:   "merge [pr-number] [target-branch]",
		Short: "Squash and merge picked PRs",
		Long: `Squash and merge picked PRs using GitHub's merge API.

This command will perform squash and merge operations on picked PRs,
equivalent to clicking the "Squash and merge" button in the GitHub UI.
Only works for PRs that are picked and have passing CI.

Examples:
  cherry-picker merge                     # Merge all eligible PRs and branches
  cherry-picker merge 123                # Merge PR #123's cherry-picks on all eligible branches
  cherry-picker merge 123 release-1.0    # Merge PR #123's cherry-pick on release-1.0`,
		Args:         cobra.RangeArgs(0, 2),
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			// Parse arguments using common utilities
			prNumber, err := commands.ParsePRNumberFromArgs(args, false)
			if err != nil {
				return err
			}
			mergeCmd.PRNumber = prNumber
			mergeCmd.TargetBranch = commands.GetTargetBranchFromArgs(args)

			// Initialize base command
			mergeCmd.ConfigFile = &configFile
			mergeCmd.LoadConfig = loadConfig
			mergeCmd.SaveConfig = saveConfig
			if err := mergeCmd.Init(cobraCmd.Context()); err != nil {
				return err
			}

			return mergeCmd.Run(cobraCmd.Context())
		},
	}

	cobraCmd.Flags().StringVar(&configFile, "config", "cherry-picks.yaml", "Path to configuration file")

	return cobraCmd
}

// Run executes the merge command
func (mc *command) Run(ctx context.Context) error {
	// If no PR number, merge all eligible PRs and branches
	if mc.PRNumber == 0 {
		return mc.mergeAllEligiblePRs(ctx)
	}

	// Find the tracked PR
	trackedPR, err := commands.FindAndValidatePR(mc.Config, mc.PRNumber)
	if err != nil {
		return err
	}

	// Validate before requiring GitHub token
	if mc.TargetBranch != "" {
		if err := commands.ValidateBranchForOperation(trackedPR, mc.TargetBranch, "merge", commands.IsEligibleForMerge); err != nil {
			return err
		}
	} else {
		if err := commands.ValidateAnyBranchForOperation(trackedPR, "merge", commands.IsEligibleForMerge); err != nil {
			return err
		}
	}

	// If target branch specified, merge only that branch
	if mc.TargetBranch != "" {
		return mc.mergeBranchPR(ctx, trackedPR, mc.TargetBranch)
	}

	// Otherwise, merge all eligible branches for this PR
	return commands.ExecuteOnEligibleBranchesForPR(
		ctx,
		trackedPR,
		"merge",
		commands.IsEligibleForMerge,
		mc.mergeBranchOperation,
		mc.Config,
		*mc.ConfigFile,
		mc.SaveConfig,
		true, // Merge operations require config saving
	)
}

// mergeBranchPR merges a specific branch's PR
func (mc *command) mergeBranchPR(ctx context.Context, trackedPR *cmd.TrackedPR, targetBranch string) error {
	err := mc.mergeBranchOperation(ctx, mc.GitHubClient, mc.Config, trackedPR, targetBranch, trackedPR.Branches[targetBranch])
	if err != nil {
		return err
	}

	// Save the updated configuration
	return mc.SaveConfig(*mc.ConfigFile, mc.Config)
}

// mergeBranchOperation is the core operation for merging a single branch
func (*command) mergeBranchOperation(ctx context.Context, client *github.Client, _ *cmd.Config, trackedPR *cmd.TrackedPR, branchName string, branchStatus cmd.BranchStatus) error {
	slog.Info("Merging PR", "original_pr", trackedPR.Number, "cherry_pick_pr", branchStatus.PR.Number, "branch", branchName)

	err := client.MergePR(ctx, branchStatus.PR.Number, "squash")
	if err != nil {
		return fmt.Errorf("failed to merge PR #%d branch %s (cherry-pick PR #%d): %w",
			trackedPR.Number, branchName, branchStatus.PR.Number, err)
	}

	// Update the branch status to merged
	branchStatus.Status = cmd.BranchStatusMerged
	trackedPR.Branches[branchName] = branchStatus

	slog.Info("Successfully merged PR", "original_pr", trackedPR.Number, "branch", branchName, "cherry_pick_pr", branchStatus.PR.Number)

	return nil
}

// mergeAllEligiblePRs merges all eligible PRs and branches across the entire config
func (mc *command) mergeAllEligiblePRs(ctx context.Context) error {
	return commands.ExecuteOnAllEligibleBranches(
		ctx,
		mc.Config,
		"merge",
		commands.IsEligibleForMerge,
		mc.mergeBranchOperation,
		*mc.ConfigFile,
		mc.SaveConfig,
		true, // Merge operations require config saving
	)
}
