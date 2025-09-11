package ignore

import (
	"fmt"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/spf13/cobra"
)

// IgnoreCommand encapsulates the ignore command with common functionality
type IgnoreCommand struct {
	commands.BaseCommand
	PRNumber     int
	TargetBranch string
}

// NewIgnoreCmd creates and returns the ignore command
func NewIgnoreCmd(globalConfigFile *string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) *cobra.Command {
	ignoreCmd := &IgnoreCommand{}

	cobraCmd := &cobra.Command{
		Use:   "ignore <pr-number> [target-branch]",
		Short: "Mark a PR as ignored for a specific target branch",
		Long: `Mark a PR as ignored for cherry-picking to a specific target branch.
If no target branch is specified, the PR will be marked as ignored for all target branches.

The PR must be currently tracked. Ignored PRs can still be picked later if needed.`,
		Args:         cobra.RangeArgs(1, 2),
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			// Parse arguments using common utilities
			prNumber, err := commands.ParsePRNumberFromArgs(args, true)
			if err != nil {
				return err
			}
			ignoreCmd.PRNumber = prNumber
			ignoreCmd.TargetBranch = commands.GetTargetBranchFromArgs(args)

			// Initialize base command
			ignoreCmd.ConfigFile = globalConfigFile
			ignoreCmd.LoadConfig = loadConfig
			ignoreCmd.SaveConfig = saveConfig
			if err := ignoreCmd.Init(); err != nil {
				return err
			}

			return ignoreCmd.Run()
		},
	}

	return cobraCmd
}

// Run executes the ignore command
func (ic *IgnoreCommand) Run() error {
	// Find and validate PR
	trackedPR, err := commands.FindAndValidatePR(ic.Config, ic.PRNumber)
	if err != nil {
		return err
	}

	// Determine branches to update
	branchesToUpdate := commands.DetermineBranchesToUpdate(trackedPR, ic.TargetBranch)
	if ic.TargetBranch != "" {
		if err := ic.validateTargetBranch(ic.TargetBranch); err != nil {
			return err
		}
	}

	// Update PR status to ignored
	ic.updatePRStatusToIgnored(trackedPR, branchesToUpdate)

	// Save configuration
	if err := ic.SaveConfigWithErrorHandling(ic.Config); err != nil {
		return err
	}

	// Display success message
	commands.DisplaySuccessMessage("ignored", ic.PRNumber, ic.TargetBranch, branchesToUpdate)
	return nil
}

// validateTargetBranch checks if the target branch exists in configuration
func (ic *IgnoreCommand) validateTargetBranch(targetBranch string) error {
	for _, branch := range ic.Config.TargetBranches {
		if branch == targetBranch {
			return nil
		}
	}
	return fmt.Errorf("target branch '%s' is not configured", targetBranch)
}

// updatePRStatusToIgnored updates the PR status to ignored for specified branches
func (ic *IgnoreCommand) updatePRStatusToIgnored(trackedPR *cmd.TrackedPR, branchesToUpdate []string) {
	if trackedPR.Branches == nil {
		trackedPR.Branches = make(map[string]cmd.BranchStatus)
	}

	for _, branch := range branchesToUpdate {
		trackedPR.Branches[branch] = cmd.BranchStatus{
			Status: cmd.BranchStatusIgnored,
			PR:     nil,
		}
	}
}
