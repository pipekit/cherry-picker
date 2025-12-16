package pick

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// launchInteractiveAIAssistant launches configured AI assistant with initial context, then hands control to user
func (pc *command) launchInteractiveAIAssistant(sha string) error {
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

	initialPrompt := pc.createInitialConflictPrompt(conflictedFiles, sha)

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
	_, _ = fmt.Scanln() // Ignore error, just waiting for Enter key

	cmd := exec.Command(pc.Config.AIAssistantCommand) //nolint:gosec // AI assistant command is user-configured
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", pc.Config.AIAssistantCommand, err)
	}

	return nil
}

// createInitialConflictPrompt creates a detailed initial prompt for the AI about the cherry-pick conflicts
func (pc *command) createInitialConflictPrompt(conflictedFiles []string, sha string) string {
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

	return prompt
}

// launchAmendAIAssistant launches AI assistant for amending an existing cherry-pick PR
func (pc *command) launchAmendAIAssistant(prNumber int, targetBranch, originalTitle string) error {
	if pc.Config.AIAssistantCommand == "" {
		return fmt.Errorf("AI assistant command not configured. Set it using: cherry-picker config --ai-assistant <command>")
	}

	prompt := pc.createAmendPrompt(prNumber, targetBranch, originalTitle)

	fmt.Printf("Amending existing cherry-pick PR #%d\n", prNumber)
	fmt.Printf("You are now on the PR's branch and can make changes.\n\n")

	separator := strings.Repeat("=", 80)
	fmt.Printf("\n%s\n", separator)
	fmt.Printf("%s\n", prompt)
	fmt.Printf("%s\n\n", separator)

	fmt.Printf("Starting %s session...\n", pc.Config.AIAssistantCommand)
	fmt.Printf("Copy the context above to start. Press Enter to launch...\n")
	_, _ = fmt.Scanln()

	cmd := exec.Command(pc.Config.AIAssistantCommand) //nolint:gosec // AI assistant command is user-configured
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", pc.Config.AIAssistantCommand, err)
	}

	return nil
}

// createAmendPrompt creates a prompt for amending an existing cherry-pick PR
func (*command) createAmendPrompt(prNumber int, targetBranch, originalTitle string) string {
	return fmt.Sprintf(`I need help amending an existing cherry-pick PR. Here's the situation:

**Context:**
- Existing cherry-pick PR: #%d
- Target branch: %s
- Original PR title: %s

**What I need:**
This cherry-pick PR was created by a bot but needs manual amendments. Common reasons include:
- CI failures that need code fixes
- Reviewer feedback requiring changes
- Additional modifications needed for this release branch

**How you can help:**
- Examine the current state of the code on this branch
- Help me understand what changes are needed
- Make the necessary modifications
- Ensure the changes are committed properly

When done, exit the session. The changes will be force-pushed to update the existing PR.`,
		prNumber, targetBranch, originalTitle)
}
