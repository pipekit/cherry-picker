// Package retry implements the retry command for retrying failed CI workflows on cherry-pick PRs.
package retry

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/alan/cherry-picker/internal/github"
	"github.com/spf13/cobra"
)

// command encapsulates the retry command with common functionality
type command struct {
	commands.BaseCommand
	PRNumber     int
	TargetBranch string
}

// NewRetryCmd creates the retry command
func NewRetryCmd(loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) *cobra.Command {
	retryCmd := &command{}
	var configFile string

	cobraCmd := &cobra.Command{
		Use:   "retry [pr-number] [target-branch]",
		Short: "Retry failed CI actions for picked PRs",
		Long: `Retry failed CI actions for picked PRs.

This command will trigger a re-run of all failed CI jobs for picked PRs.
Only works for PRs with failed CI status.

Examples:
  cherry-picker retry                     # Retry failed CI for all eligible PRs and branches
  cherry-picker retry 123                # Retry failed CI for PR #123 on all branches
  cherry-picker retry 123 release-1.0    # Retry failed CI for PR #123 on release-1.0`,
		Args:         cobra.RangeArgs(0, 2),
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			// Parse arguments using common utilities
			prNumber, err := commands.ParsePRNumberFromArgs(args, false)
			if err != nil {
				return err
			}
			retryCmd.PRNumber = prNumber
			retryCmd.TargetBranch = commands.GetTargetBranchFromArgs(args)

			// Initialize base command
			retryCmd.ConfigFile = &configFile
			retryCmd.LoadConfig = loadConfig
			retryCmd.SaveConfig = saveConfig
			if err := retryCmd.Init(cobraCmd.Context()); err != nil {
				return err
			}

			return retryCmd.Run(cobraCmd.Context())
		},
	}

	cobraCmd.Flags().StringVar(&configFile, "config", "cherry-picks.yaml", "Path to configuration file")

	return cobraCmd
}

// Run executes the retry command
func (rc *command) Run(ctx context.Context) error {
	// If no PR number, retry all eligible PRs and branches
	if rc.PRNumber == 0 {
		return rc.retryAllEligiblePRs(ctx)
	}

	// Find the tracked PR
	trackedPR, err := commands.FindAndValidatePR(rc.Config, rc.PRNumber)
	if err != nil {
		return err
	}

	// Validate before requiring GitHub token
	if rc.TargetBranch != "" {
		if err := commands.ValidateBranchForOperation(trackedPR, rc.TargetBranch, "retry", commands.IsEligibleForRetry); err != nil {
			return err
		}
	} else {
		if err := commands.ValidateAnyBranchForOperation(trackedPR, "retry", commands.IsEligibleForRetry); err != nil {
			return err
		}
	}

	// If target branch specified, retry only that branch
	if rc.TargetBranch != "" {
		return rc.retryBranchCI(ctx, trackedPR, rc.TargetBranch)
	}

	// Otherwise, retry all picked branches with failed CI for this PR
	return commands.ExecuteOnEligibleBranchesForPR(
		ctx,
		trackedPR,
		"retry",
		commands.IsEligibleForRetry,
		rc.retryBranchOperation,
		rc.Config,
		*rc.ConfigFile,
		nil, // No config saving needed for retry
		false,
	)
}

// retryBranchCI retries CI for a specific branch
func (rc *command) retryBranchCI(ctx context.Context, trackedPR *cmd.TrackedPR, targetBranch string) error {
	return rc.retryBranchOperation(ctx, rc.GitHubClient, rc.Config, trackedPR, targetBranch, trackedPR.Branches[targetBranch])
}

// retryBranchOperation is the core operation for retrying CI on a single branch
func (*command) retryBranchOperation(ctx context.Context, client *github.Client, _ *cmd.Config, trackedPR *cmd.TrackedPR, branchName string, branchStatus cmd.BranchStatus) error {
	slog.Info("Retrying failed CI for PR", "original_pr", trackedPR.Number, "cherry_pick_pr", branchStatus.PR.Number, "branch", branchName)

	err := client.RetryFailedWorkflows(ctx, branchStatus.PR.Number)
	if err != nil {
		return fmt.Errorf("failed to retry CI for PR #%d branch %s (cherry-pick PR #%d): %w",
			trackedPR.Number, branchName, branchStatus.PR.Number, err)
	}

	slog.Info("Successfully triggered retry for failed CI jobs", "original_pr", trackedPR.Number, "branch", branchName, "cherry_pick_pr", branchStatus.PR.Number)

	return nil
}

// retryAllEligiblePRs retries CI for all eligible PRs and branches across the entire config
func (rc *command) retryAllEligiblePRs(ctx context.Context) error {
	return commands.ExecuteOnAllEligibleBranches(
		ctx,
		rc.Config,
		"retry",
		commands.IsEligibleForRetry,
		rc.retryBranchOperation,
		*rc.ConfigFile,
		nil, // No config saving needed for retry
		false,
	)
}
