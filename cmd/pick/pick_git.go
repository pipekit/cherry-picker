package pick

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// performGitFetch fetches the latest changes from remote
func (pc *PickCommand) performGitFetch() error {
	slog.Info("Fetching latest changes from remote")
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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

// performCherryPick executes the git cherry-pick command with AI integration for conflicts
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

// pushBranch pushes a branch to origin
func (pc *PickCommand) pushBranch(branchName string) error {
	slog.Info("Pushing branch", "branch", branchName)
	cmd := exec.Command("git", "push", "origin", branchName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
