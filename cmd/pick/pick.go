// Package pick implements the pick command for AI-assisted cherry-picking of PRs that bots couldn't handle.
package pick

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/spf13/cobra"
)

// command encapsulates the pick command with common functionality
type command struct {
	commands.BaseCommand
	PRNumber     int
	TargetBranch string
	Force        bool
}

// NewPickCmd creates and returns the pick command
func NewPickCmd(globalConfigFile *string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) *cobra.Command {
	pickCmd := &command{}

	cobraCmd := &cobra.Command{
		Use:   "pick <pr-number> [target-branch]",
		Short: "AI-assisted cherry-pick for PRs that bots couldn't handle",
		Long: `Cherry-pick a PR to target branches with AI-assisted conflict resolution.
This command is for handling cherry-picks that the automated bot couldn't complete due to conflicts.

If no target branch is specified, the PR will be cherry-picked to all failed branches.
The PR must be currently tracked and have 'failed' status for the target branch(es).

Use --force to amend an existing bot-created cherry-pick PR that has 'picked' status.
This fetches the existing PR branch, allows AI-assisted modifications, and force pushes.

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
			if err := pickCmd.Init(cobraCmd.Context()); err != nil {
				return err
			}

			return pickCmd.Run(cobraCmd.Context())
		},
	}

	cobraCmd.Flags().BoolVar(&pickCmd.Force, "force", false, "Amend existing cherry-pick PR instead of creating new one")

	return cobraCmd
}

// Run executes the pick command
func (pc *command) Run(ctx context.Context) error {
	return pc.runPick(ctx)
}

// runPick executes the full cherry-pick workflow
func (pc *command) runPick(ctx context.Context) error {
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

	// Get commit SHA only in normal mode (not needed for force amend)
	var sha string
	if !pc.Force {
		var err error
		sha, err = pc.getCommitSHA(ctx, pc.PRNumber)
		if err != nil {
			return err
		}
	}

	if err := pc.performGitFetch(); err != nil {
		return fmt.Errorf("failed to fetch from remote: %w", err)
	}

	// Perform cherry-pick (or force amend) for each branch with immediate saving
	for _, branch := range branches {
		var result *CherryPickResult
		var err error

		if pc.Force {
			// Force mode: amend existing cherry-pick PR
			result, err = pc.performForceAmendForBranch(ctx, branch, pr)
		} else {
			// Normal mode: cherry-pick from scratch
			result, err = pc.performCherryPickForBranch(ctx, sha, branch, pc.PRNumber, pr.Title)
		}

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
func (pc *command) runPickForTest() error {
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

// validatePickableStatus validates branches can be picked (must be in 'failed' status, or 'picked' with --force)
func (pc *command) validatePickableStatus(pr *cmd.TrackedPR, branches []string) error {
	if pr.Branches == nil {
		pr.Branches = make(map[string]cmd.BranchStatus)
	}

	for _, branch := range branches {
		status, exists := pr.Branches[branch]
		if !exists {
			return fmt.Errorf("PR #%d has no status for branch '%s'", pc.PRNumber, branch)
		}

		// Handle --force mode: require 'picked' status with existing PR
		if pc.Force {
			if status.Status != cmd.BranchStatusPicked {
				return fmt.Errorf("--force requires PR #%d branch '%s' to have 'picked' status (current: %s)", pc.PRNumber, branch, status.Status)
			}
			if status.PR == nil || status.PR.Number == 0 {
				return fmt.Errorf("--force requires existing cherry-pick PR for PR #%d branch '%s'", pc.PRNumber, branch)
			}
			continue
		}

		// Normal mode validation
		switch status.Status {
		case cmd.BranchStatusPicked, cmd.BranchStatusMerged, cmd.BranchStatusReleased:
			return fmt.Errorf("PR #%d for branch '%s' can only be picked if bot cherry-pick failed or pending (current status: %s)", pc.PRNumber, branch, status.Status)
		case cmd.BranchStatusPending:
			// Bot hasn't attempted yet - ask user to confirm
			fmt.Printf("⚠️  PR #%d for branch '%s' is still pending (bot hasn't attempted cherry-pick yet).\n", pc.PRNumber, branch)
			fmt.Printf("Are you sure you want to pick manually? (y/N): ")

			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read user input: %w", err)
			}

			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				return fmt.Errorf("aborted: PR #%d for branch '%s' is pending - wait for bot to attempt first", pc.PRNumber, branch)
			}
		case cmd.BranchStatusFailed:
			// This is expected
		}
	}
	return nil
}

// updatePRStatus updates the PR status to picked for specified branches
func (*command) updatePRStatus(pr *cmd.TrackedPR, branches []string) {
	for _, branch := range branches {
		pr.Branches[branch] = cmd.BranchStatus{
			Status: cmd.BranchStatusPicked,
			PR:     nil, // Will be set later when we know the actual pick PR number and details
		}
	}
}

// updateSingleBranchStatus updates PR status for a single branch with cherry-pick result
func (*command) updateSingleBranchStatus(pr *cmd.TrackedPR, branch string, result *CherryPickResult) {
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
func (pc *command) performCherryPickForBranch(ctx context.Context, sha, branch string, prNumber int, originalTitle string) (*CherryPickResult, error) {
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

	cherryPickPRNumber, err := pc.createCherryPickPR(ctx, cherryPickBranch, branch, prNumber, originalTitle)
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

// performForceAmendForBranch fetches and amends an existing cherry-pick PR
func (pc *command) performForceAmendForBranch(ctx context.Context, branch string, trackedPR *cmd.TrackedPR) (*CherryPickResult, error) {
	branchStatus := trackedPR.Branches[branch]
	existingPRNumber := branchStatus.PR.Number

	fmt.Printf("Fetching existing cherry-pick PR #%d for branch %s...\n", existingPRNumber, branch)

	// Fetch the existing PR's branch
	if err := pc.fetchPRBranch(existingPRNumber); err != nil {
		return nil, fmt.Errorf("failed to fetch PR #%d branch: %w", existingPRNumber, err)
	}

	// Get the remote branch name for force pushing later
	remoteBranch, err := pc.GitHubClient.GetPRHeadBranch(ctx, existingPRNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR #%d head branch: %w", existingPRNumber, err)
	}

	// Launch AI assistant with amend-specific prompt
	if err := pc.launchAmendAIAssistant(existingPRNumber, branch, trackedPR.Title); err != nil {
		return nil, fmt.Errorf("AI assistant session failed: %w", err)
	}

	// Force push to update the existing PR
	localBranch := fmt.Sprintf("pr-%d", existingPRNumber)
	if err := pc.forcePushBranch(localBranch, remoteBranch); err != nil {
		return nil, fmt.Errorf("failed to force push: %w", err)
	}

	fmt.Printf("Updated existing PR #%d for branch: %s\n", existingPRNumber, branch)

	// Return result preserving existing PR info (CI will re-run after push)
	return &CherryPickResult{
		PRNumber: existingPRNumber,
		Title:    branchStatus.PR.Title,
		CIStatus: "pending",
	}, nil
}
