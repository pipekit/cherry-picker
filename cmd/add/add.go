package add

import (
	"fmt"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/spf13/cobra"
)

// AddCommand encapsulates the add command with common functionality
type AddCommand struct {
	commands.BaseCommand
	PRNumber     int
	TargetBranch string
}

// NewAddCmd creates and returns the add command
func NewAddCmd(globalConfigFile *string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) *cobra.Command {
	addCmd := &AddCommand{}

	cobraCmd := &cobra.Command{
		Use:   "add <pr-number> [target-branch]",
		Short: "Add a PR to the tracking list",
		Long: `Add a PR to the cherry-pick tracking list.

This command fetches PR details from GitHub and adds it to the configuration
for tracking. If a target branch is specified, the PR will be marked as pending
for that branch only. If no target branch is specified, the PR will be marked
as pending for all configured target branches.

The PR must exist and be accessible via the GitHub API.`,
		Args:         cobra.RangeArgs(1, 2),
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			// Parse arguments using common utilities
			prNumber, err := commands.ParsePRNumberFromArgs(args, true)
			if err != nil {
				return err
			}
			addCmd.PRNumber = prNumber
			addCmd.TargetBranch = commands.GetTargetBranchFromArgs(args)

			// Initialize base command
			addCmd.ConfigFile = globalConfigFile
			addCmd.LoadConfig = loadConfig
			addCmd.SaveConfig = saveConfig
			if err := addCmd.Init(); err != nil {
				return err
			}

			return addCmd.Run()
		},
	}

	return cobraCmd
}

// Run executes the add command
func (ac *AddCommand) Run() error {
	// Check if PR is already tracked
	if ac.isPRAlreadyTracked() {
		return fmt.Errorf("PR #%d is already being tracked", ac.PRNumber)
	}

	// Initialize GitHub client
	client, _, err := commands.InitializeGitHubClient()
	if err != nil {
		return err
	}

	// Fetch PR details from GitHub
	prDetails, err := client.GetPRWithDetails(ac.Config.Org, ac.Config.Repo, ac.PRNumber)
	if err != nil {
		return fmt.Errorf("failed to fetch PR details: %w", err)
	}

	// Create new tracked PR
	newTrackedPR := cmd.TrackedPR{
		Number:   ac.PRNumber,
		Title:    prDetails.Title,
		Ignored:  false,
		Branches: make(map[string]cmd.BranchStatus),
	}

	// Determine which branches to add
	branchesToAdd, err := ac.determineBranchesToAdd()
	if err != nil {
		return err
	}

	// Add pending status for each target branch
	for _, branch := range branchesToAdd {
		newTrackedPR.Branches[branch] = cmd.BranchStatus{
			Status: cmd.BranchStatusPending,
		}
	}

	// Add the PR to the configuration
	ac.Config.TrackedPRs = append(ac.Config.TrackedPRs, newTrackedPR)

	// Save the updated configuration
	if err := ac.SaveConfig(*ac.ConfigFile, ac.Config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Display success message
	ac.displaySuccessMessage(prDetails.Title, branchesToAdd)

	return nil
}

// isPRAlreadyTracked checks if the PR is already being tracked
func (ac *AddCommand) isPRAlreadyTracked() bool {
	for _, trackedPR := range ac.Config.TrackedPRs {
		if trackedPR.Number == ac.PRNumber {
			return true
		}
	}
	return false
}

// determineBranchesToAdd determines which branches to add the PR to
func (ac *AddCommand) determineBranchesToAdd() ([]string, error) {
	if ac.TargetBranch != "" {
		// Validate the target branch exists in config
		for _, branch := range ac.Config.TargetBranches {
			if branch == ac.TargetBranch {
				return []string{ac.TargetBranch}, nil
			}
		}
		// If target branch not found in config, return error
		return nil, fmt.Errorf("target branch '%s' not found in configuration. Available branches: %v", ac.TargetBranch, ac.Config.TargetBranches)
	}
	// If no target branch specified, add to all configured target branches
	return ac.Config.TargetBranches, nil
}

// displaySuccessMessage shows a success message after adding the PR
func (ac *AddCommand) displaySuccessMessage(prTitle string, branches []string) {
	fmt.Printf("✅ Successfully added PR #%d to tracking list\n", ac.PRNumber)
	fmt.Printf("   Title: %s\n", prTitle)
	fmt.Printf("   Branches: %v\n", branches)
	fmt.Printf("   Status: pending for all specified branches\n")
}
