package pick

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/spf13/cobra"
)

// CherryPickResult holds the result of a cherry-pick operation
type CherryPickResult struct {
	PRNumber int
	Title    string
	CIStatus string
}

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

// Git operation methods

// performGitFetch fetches the latest changes from remote
func (pc *PickCommand) performGitFetch() error {
	slog.Info("Fetching latest changes from remote")
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// getCommitSHA retrieves the merge commit SHA for a PR
func (pc *PickCommand) getCommitSHA(prNumber int) (string, error) {
	pr, err := pc.GitHubClient.GetPR(prNumber)
	if err != nil {
		return "", fmt.Errorf("failed to get PR details: %w", err)
	}

	if pr.SHA == "" {
		return "", fmt.Errorf("PR #%d has no merge commit SHA", prNumber)
	}

	return pr.SHA, nil
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

// checkoutBranch switches to the target branch and force updates it to match upstream
func (pc *PickCommand) checkoutBranch(branch string) error {
	slog.Info("Checking out branch", "branch", branch)

	checkoutCmd := exec.Command("git", "checkout", branch)
	checkoutCmd.Stdout = os.Stdout
	checkoutCmd.Stderr = os.Stderr
	if err := checkoutCmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout branch %s: %w", branch, err)
	}

	slog.Info("Updating branch to match upstream", "branch", branch, "upstream", fmt.Sprintf("origin/%s", branch))
	resetCmd := exec.Command("git", "reset", "--hard", fmt.Sprintf("origin/%s", branch))
	resetCmd.Stdout = os.Stdout
	resetCmd.Stderr = os.Stderr
	if err := resetCmd.Run(); err != nil {
		return fmt.Errorf("failed to reset branch %s to upstream: %w", branch, err)
	}

	return nil
}

// createAndCheckoutBranch creates a new branch and checks it out, recreating if it already exists
func (pc *PickCommand) createAndCheckoutBranch(branchName string) error {
	slog.Info("Creating and checking out branch", "branch", branchName)

	// Delete local branch if it exists
	deleteLocalCmd := exec.Command("git", "branch", "-D", branchName)
	deleteLocalCmd.Run()

	// Delete remote branch if it exists
	deleteRemoteCmd := exec.Command("git", "push", "origin", "--delete", branchName)
	deleteRemoteCmd.Run()

	// Create and checkout the new branch
	cmd := exec.Command("git", "checkout", "-b", branchName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// pushBranch pushes a branch to origin
func (pc *PickCommand) pushBranch(branchName string) error {
	slog.Info("Pushing branch", "branch", branchName)
	cmd := exec.Command("git", "push", "origin", branchName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// createCherryPickPR creates a PR for the cherry-pick using bot-style formatting
func (pc *PickCommand) createCherryPickPR(headBranch, baseBranch string, originalPRNumber int, originalTitle string) (int, error) {
	// Extract version from branch name (e.g., "release-3.7" -> "3.7")
	version := strings.TrimPrefix(baseBranch, "release-")

	// Title format matches bot: "<original-title> (cherry-pick #<pr> for <version>)"
	prTitle := fmt.Sprintf("%s (cherry-pick #%d for %s)", originalTitle, originalPRNumber, version)

	// Body format matches bot: "Cherry-picked <original-title> (#<pr>)"
	prDescription := fmt.Sprintf("Cherry-picked %s (#%d)", originalTitle, originalPRNumber)

	prNumber, err := pc.GitHubClient.CreatePR(prTitle, prDescription, headBranch, baseBranch)
	if err != nil {
		return 0, fmt.Errorf("GitHub API error creating PR from %s to %s: %w", headBranch, baseBranch, err)
	}

	fmt.Printf("📝 Created PR #%d: %s\n", prNumber, prTitle)
	return prNumber, nil
}

// performCherryPick executes the git cherry-pick command with Cursor integration for conflicts
func (pc *PickCommand) performCherryPick(sha string) error {
	slog.Info("Cherry-picking commit", "sha", sha)
	cmd := exec.Command("git", "cherry-pick", "-x", "--signoff", sha)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if pc.isConflictError(err) {
			slog.Warn("Cherry-pick conflicts detected, attempting AI-assisted resolution")

			if resolveErr := pc.launchInteractiveAIAssistant(sha); resolveErr != nil {
				slog.Error("Failed to launch AI assistant", "error", resolveErr)
				fmt.Printf("   - You can resolve conflicts manually using standard Git tools\n")
				fmt.Printf("   - Run 'git cherry-pick --abort' to cancel, or resolve and 'git cherry-pick --continue'\n")
				return fmt.Errorf("cherry-pick failed and AI assistant launch failed: %w (original: %v)", resolveErr, err)
			}

			slog.Info("AI assistant session completed")
			fmt.Println("   - Assuming conflicts have been resolved during the AI session")
			fmt.Println("   - Checking if cherry-pick is complete...")

			remainingConflicts, err := pc.getConflictedFiles()
			if err != nil {
				return fmt.Errorf("failed to check for remaining conflicts: %w", err)
			}

			if len(remainingConflicts) > 0 {
				slog.Warn("Files still have conflicts after AI session", "conflicted_files", remainingConflicts)
				fmt.Println("   - Please resolve these manually and run 'git cherry-pick --continue'")
				fmt.Println("   - Or run 'git cherry-pick --abort' to cancel")
				return fmt.Errorf("conflicts still remain after AI session")
			}

			if _, err := os.Stat(".git/CHERRY_PICK_HEAD"); os.IsNotExist(err) {
				slog.Info("Cherry-pick appears to be already complete")
				return nil
			}

			slog.Info("No conflicts remaining, completing cherry-pick commit")
			continueCmd := exec.Command("git", "cherry-pick", "--continue")
			continueCmd.Stdout = os.Stdout
			continueCmd.Stderr = os.Stderr
			if continueErr := continueCmd.Run(); continueErr != nil {
				return fmt.Errorf("failed to complete cherry-pick: %w", continueErr)
			}

			slog.Info("Cherry-pick completed with AI-assisted conflict resolution")
			return nil
		}

		return err
	}

	return nil
}

// isConflictError checks if the error is due to merge conflicts
func (pc *PickCommand) isConflictError(err error) bool {
	if err == nil {
		return false
	}

	if exitError, ok := err.(*exec.ExitError); ok {
		return exitError.ExitCode() == 1
	}

	return false
}

// launchInteractiveAIAssistant launches configured AI assistant with initial context, then hands control to user
func (pc *PickCommand) launchInteractiveAIAssistant(sha string) error {
	if pc.Config.AIAssistantCommand == "" {
		return fmt.Errorf("AI assistant command not configured. Set it using: cherry-picker config --ai-assistant <command>")
	}

	conflictedFiles, err := pc.getConflictedFiles()
	if err != nil {
		return fmt.Errorf("failed to get conflicted files: %w", err)
	}

	if len(conflictedFiles) == 0 {
		return fmt.Errorf("no conflicted files found")
	}

	slog.Info("Found conflicted files", "count", len(conflictedFiles), "files", conflictedFiles)
	slog.Info("Launching AI assistant with initial context", "command", pc.Config.AIAssistantCommand)

	initialPrompt, err := pc.createInitialConflictPrompt(conflictedFiles, sha)
	if err != nil {
		return fmt.Errorf("failed to create initial prompt: %w", err)
	}

	fmt.Printf("💡 Starting AI session with conflict context, then handing control to you.\n")
	fmt.Printf("   - The AI will receive details about the cherry-pick conflicts\n")
	fmt.Printf("   - You can then guide the resolution process\n")
	fmt.Printf("   - Exit the agent when you're satisfied with the resolution\n\n")

	slog.Info("Sending initial context to AI")
	separator := strings.Repeat("=", 80)
	fmt.Printf("\n%s\n", separator)
	fmt.Printf("%s\n", initialPrompt)
	fmt.Printf("%s\n\n", separator)

	fmt.Printf("🤖 Starting %s session...\n", pc.Config.AIAssistantCommand)
	fmt.Printf("💡 Copy the context above and paste it to start the conversation with the AI.\n")
	fmt.Printf("   Press Enter to launch %s...\n", pc.Config.AIAssistantCommand)
	fmt.Scanln()

	cmd := exec.Command(pc.Config.AIAssistantCommand)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", pc.Config.AIAssistantCommand, err)
	}

	return nil
}

// createInitialConflictPrompt creates a detailed initial prompt for the AI about the cherry-pick conflicts
func (pc *PickCommand) createInitialConflictPrompt(conflictedFiles []string, sha string) (string, error) {
	commitInfo, err := pc.getCommitInfo(sha)
	if err != nil {
		commitInfo = fmt.Sprintf("commit %s", sha[:8])
	}

	prompt := fmt.Sprintf(`I need help resolving cherry-pick conflicts. Here's the situation:

**Cherry-pick Context:**
- Attempting to cherry-pick: %s
- Number of conflicted files: %d
- Conflicted files: %v

**What I need:**
I'd like you to help me resolve these merge conflicts. You can see the conflicted files in the repository. Each file contains Git conflict markers (<<<<<<< HEAD, =======, >>>>>>> %s) that need to be resolved.

**How you can help:**
- Analyze the conflicts in each file
- Explain what's conflicting and why
- Suggest resolution strategies
- Help me understand the best way to merge these changes
- Make the actual changes when I ask you to

Please start by examining the conflicted files and let me know what you see.`,
		commitInfo, len(conflictedFiles), conflictedFiles, sha[:8])

	return prompt, nil
}

// getCommitInfo gets a human-readable description of a commit
func (pc *PickCommand) getCommitInfo(sha string) (string, error) {
	cmd := exec.Command("git", "log", "--oneline", "-1", sha)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getConflictedFiles returns a list of files with merge conflicts
func (pc *PickCommand) getConflictedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	var conflictedFiles []string
	for _, file := range files {
		if file != "" {
			conflictedFiles = append(conflictedFiles, file)
		}
	}

	return conflictedFiles, nil
}

// moveSignedOffByLinesToEnd ensures Signed-off-by lines are at the end of the commit message
func (pc *PickCommand) moveSignedOffByLinesToEnd() error {
	getMessageCmd := exec.Command("git", "log", "-1", "--pretty=format:%B")
	messageBytes, err := getMessageCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get commit message: %w", err)
	}

	originalMessage := strings.TrimSpace(string(messageBytes))
	if originalMessage == "" {
		return nil
	}

	lines := strings.Split(originalMessage, "\n")
	var bodyLines []string
	var signedOffByLines []string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "Signed-off-by:") {
			signedOffByLines = append(signedOffByLines, line)
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	if len(signedOffByLines) == 0 {
		return nil
	}

	// Remove trailing empty lines from body
	for len(bodyLines) > 0 && strings.TrimSpace(bodyLines[len(bodyLines)-1]) == "" {
		bodyLines = bodyLines[:len(bodyLines)-1]
	}

	var newMessage strings.Builder

	for i, line := range bodyLines {
		newMessage.WriteString(line)
		if i < len(bodyLines)-1 {
			newMessage.WriteString("\n")
		}
	}

	if len(bodyLines) > 0 && len(signedOffByLines) > 0 {
		newMessage.WriteString("\n\n")
	}

	for i, line := range signedOffByLines {
		newMessage.WriteString(line)
		if i < len(signedOffByLines)-1 {
			newMessage.WriteString("\n")
		}
	}

	finalMessage := newMessage.String()

	if len(signedOffByLines) > 0 {
		slog.Info("Found Signed-off-by lines", "count", len(signedOffByLines))
		for _, line := range signedOffByLines {
			fmt.Printf("   %s\n", strings.TrimSpace(line))
		}
	}

	if finalMessage != originalMessage {
		slog.Info("Moving Signed-off-by lines to end of commit message")

		amendCmd := exec.Command("git", "commit", "--amend", "-m", finalMessage)
		amendCmd.Stdout = os.Stdout
		amendCmd.Stderr = os.Stderr

		if err := amendCmd.Run(); err != nil {
			return fmt.Errorf("failed to amend commit message: %w", err)
		}
	}

	return nil
}
