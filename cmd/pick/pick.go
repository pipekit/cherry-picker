package pick

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/spf13/cobra"
)

// PickCommand encapsulates the pick command with common functionality
type PickCommand struct {
	commands.BaseCommand
	PRNumber     int
	TargetBranch string
}

// NewPickCmd creates and returns the pick command
func NewPickCmd(globalConfigFile *string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) *cobra.Command {
	pickCmd := &PickCommand{}

	cobraCmd := &cobra.Command{
		Use:   "pick <pr-number> [target-branch]",
		Short: "AI-assisted cherry-pick for PRs that bots couldn't handle",
		Long: `Cherry-pick a PR to target branches with AI-assisted conflict resolution.
This command is for handling cherry-picks that the automated bot couldn't complete due to conflicts.

If no target branch is specified, the PR will be cherry-picked to all failed branches.
The PR must be currently tracked and have 'failed' status for the target branch(es).

Conflicts are automatically resolved using configured AI assistant.`,
		Args:         cobra.RangeArgs(1, 2),
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			// Parse arguments using common utilities
			prNumber, err := commands.ParsePRNumberFromArgs(args, true)
			if err != nil {
				return err
			}
			pickCmd.PRNumber = prNumber
			pickCmd.TargetBranch = commands.GetTargetBranchFromArgs(args)

			// Initialize base command
			pickCmd.ConfigFile = globalConfigFile
			pickCmd.LoadConfig = loadConfig
			pickCmd.SaveConfig = saveConfig
			if err := pickCmd.Init(); err != nil {
				return err
			}

			return pickCmd.Run()
		},
	}

	return cobraCmd
}

// Run executes the pick command
func (pc *PickCommand) Run() error {
	return pc.runPick()
}

// runPick executes the full cherry-pick workflow
func (pc *PickCommand) runPick() error {
	// Find and validate PR (4 lines vs ~15 lines)
	pr, err := commands.FindAndValidatePR(pc.Config, pc.PRNumber)
	if err != nil {
		return err
	}

	// Determine branches to update (3 lines vs ~10 lines)
	branches := commands.DetermineBranchesToUpdate(pr, pc.TargetBranch)

	// Validate branch status
	if err := pc.validatePickableStatus(pr, branches); err != nil {
		return err
	}

	// Git operations
	if err := commands.ValidateGitRepository(); err != nil {
		return err
	}

	sha, err := pc.getCommitSHA(pc.PRNumber)
	if err != nil {
		return err
	}

	if err := pc.performGitFetch(); err != nil {
		return fmt.Errorf("failed to fetch from remote: %w", err)
	}

	// Perform cherry-pick for each branch with immediate saving
	for _, branch := range branches {
		result, err := pc.performCherryPickForBranch(sha, branch, pc.PRNumber, pr.Title)
		if err != nil {
			return err
		}

		// Update and save immediately after each successful cherry-pick
		pc.updateSingleBranchStatus(pr, branch, result)
		if err := pc.SaveConfig(*pc.ConfigFile, pc.Config); err != nil {
			slog.Warn("Failed to save config after successful cherry-pick", "branch", branch, "error", err)
		} else {
			slog.Info("Saved progress for branch", "branch", branch)
		}
	}

	// Final save and display
	if err := pc.SaveConfig(*pc.ConfigFile, pc.Config); err != nil {
		return err
	}

	commands.DisplaySuccessMessage("picked", pc.PRNumber, pc.TargetBranch, branches)
	return nil
}

// runPickForTest executes pick for testing without git operations
func (pc *PickCommand) runPickForTest() error {
	// Find and validate PR
	pr, err := commands.FindAndValidatePR(pc.Config, pc.PRNumber)
	if err != nil {
		return err
	}

	// Determine branches to update
	branches := commands.DetermineBranchesToUpdate(pr, pc.TargetBranch)

	// Validate branch status
	if err := pc.validatePickableStatus(pr, branches); err != nil {
		return err
	}

	// Mark as picked without git operations (for testing)
	pc.updatePRStatus(pr, branches)

	// Save config
	if err := pc.SaveConfig(*pc.ConfigFile, pc.Config); err != nil {
		return err
	}

	commands.DisplaySuccessMessage("picked", pc.PRNumber, pc.TargetBranch, branches)
	return nil
}

// validatePickableStatus validates branches can be picked (must be in 'failed' status)
func (pc *PickCommand) validatePickableStatus(pr *cmd.TrackedPR, branches []string) error {
	if pr.Branches == nil {
		pr.Branches = make(map[string]cmd.BranchStatus)
	}

	for _, branch := range branches {
		status, exists := pr.Branches[branch]
		if !exists {
			return fmt.Errorf("PR #%d has no status for branch '%s'", pc.PRNumber, branch)
		}
		if status.Status != cmd.BranchStatusFailed {
			return fmt.Errorf("PR #%d for branch '%s' can only be picked if bot cherry-pick failed (current status: %s, expected: failed)", pc.PRNumber, branch, status.Status)
		}
	}
	return nil
}

// updatePRStatus updates the PR status to picked for specified branches
func (pc *PickCommand) updatePRStatus(pr *cmd.TrackedPR, branches []string) {
	for _, branch := range branches {
		pr.Branches[branch] = cmd.BranchStatus{
			Status: cmd.BranchStatusPicked,
			PR:     nil, // Will be set later when we know the actual pick PR number and details
		}
	}
}

// updateSingleBranchStatus updates PR status for a single branch with cherry-pick result
func (pc *PickCommand) updateSingleBranchStatus(pr *cmd.TrackedPR, branch string, result *CherryPickResult) {
	pr.Branches[branch] = cmd.BranchStatus{
		Status: cmd.BranchStatusPicked,
		PR: &cmd.PickPR{
			Number:   result.PRNumber,
			Title:    result.Title,
			CIStatus: cmd.ParseCIStatus(result.CIStatus),
		},
	}
}

// performCherryPickForBranch performs cherry-pick for a specific branch
func (pc *PickCommand) performCherryPickForBranch(sha, branch string, prNumber int, originalTitle string) (*CherryPickResult, error) {
	cherryPickBranch := fmt.Sprintf("cherry-pick-%d-%s", prNumber, branch)

	if err := pc.checkoutBranch(branch); err != nil {
		return nil, err
	}

	if err := pc.createAndCheckoutBranch(cherryPickBranch); err != nil {
		return nil, fmt.Errorf("failed to create branch %s: %w", cherryPickBranch, err)
	}

	if err := pc.performCherryPick(sha); err != nil {
		return nil, fmt.Errorf("git cherry-pick failed for commit %s: %w", sha[:8], err)
	}

	if err := pc.moveSignedOffByLinesToEnd(); err != nil {
		return nil, fmt.Errorf("failed to reorder Signed-off-by lines: %w", err)
	}

	if err := pc.pushBranch(cherryPickBranch); err != nil {
		return nil, fmt.Errorf("git push failed for branch %s: %w", cherryPickBranch, err)
	}

	cherryPickPRNumber, err := pc.createCherryPickPR(cherryPickBranch, branch, prNumber, originalTitle)
	if err != nil {
		return nil, err
	}

	fmt.Printf("✅ Successfully cherry-picked to branch: %s\n", branch)
	fmt.Printf("✅ Created PR #%d: %s → %s\n", cherryPickPRNumber, cherryPickBranch, branch)

	// Extract version from branch name for title (e.g., "release-3.7" -> "3.7")
	version := strings.TrimPrefix(branch, "release-")
	prTitle := fmt.Sprintf("%s (cherry-pick #%d for %s)", originalTitle, prNumber, version)

	return &CherryPickResult{
		PRNumber: cherryPickPRNumber,
		Title:    prTitle,
		CIStatus: "pending",
	}, nil
}
