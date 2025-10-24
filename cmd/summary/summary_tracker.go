package summary

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/alan/cherry-picker/internal/github"
)

// postToTrackerIssue handles posting or updating the summary as a comment on the tracker issue
func (sc *command) postToTrackerIssue(ctx context.Context, version, summary string) error {
	// Get tracker issue number for this branch
	trackerIssue, exists := sc.Config.TrackerIssues[sc.TargetBranch]
	if !exists {
		return fmt.Errorf("no tracker issue found for branch %s", sc.TargetBranch)
	}

	// Get authenticated user
	username, err := sc.GitHubClient.GetAuthenticatedUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get authenticated user: %w", err)
	}

	// Get existing comments on the tracker issue
	comments, err := sc.GitHubClient.GetIssueComments(ctx, trackerIssue)
	if err != nil {
		return fmt.Errorf("failed to get issue comments: %w", err)
	}

	// Look for existing comment by this user with this version title
	existingComment := findExistingComment(comments, username, version)

	if existingComment != nil {
		// Check if there's any difference
		diff := generateDiff(existingComment.Body, summary)
		if diff == "" {
			fmt.Println("\nNo changes to post - existing comment is identical.")
			return nil
		}

		// Show diff and confirm update
		fmt.Println("\nExisting comment found. Diff:")
		fmt.Println(diff)

		if !confirmAction("Update the existing comment?") {
			fmt.Println("Update cancelled.")
			return nil
		}

		// Update comment
		_, err := sc.GitHubClient.UpdateIssueComment(ctx, existingComment.ID, summary)
		if err != nil {
			return fmt.Errorf("failed to update comment: %w", err)
		}

		fmt.Printf("\nComment updated successfully on issue #%d\n", trackerIssue)
	} else {
		// Confirm creating new comment
		fmt.Printf("\nPost this summary as a comment on tracker issue #%d?\n", trackerIssue)
		if !confirmAction("Post comment?") {
			fmt.Println("Posting cancelled.")
			return nil
		}

		// Create new comment
		_, err := sc.GitHubClient.CreateIssueComment(ctx, trackerIssue, summary)
		if err != nil {
			return fmt.Errorf("failed to create comment: %w", err)
		}

		fmt.Printf("\nComment posted successfully on issue #%d\n", trackerIssue)
	}

	return nil
}

// findExistingComment looks for a comment by the given user that starts with the version title
func findExistingComment(comments []github.Comment, username, version string) *github.Comment {
	versionTitle := fmt.Sprintf("### %s:", version)

	for i := range comments {
		comment := &comments[i]
		if comment.User == username && strings.HasPrefix(strings.TrimSpace(comment.Body), versionTitle) {
			return comment
		}
	}

	return nil
}

// generateDiff generates a simple diff between old and new content
func generateDiff(oldContent, newContent string) string {
	// Trim whitespace for comparison
	oldTrimmed := strings.TrimSpace(oldContent)
	newTrimmed := strings.TrimSpace(newContent)

	if oldTrimmed == newTrimmed {
		return ""
	}

	// Simple diff showing removed and added lines
	var diff strings.Builder

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Show side-by-side comparison
	diff.WriteString("--- Old\n")
	diff.WriteString("+++ New\n\n")

	maxLen := max(len(oldLines), len(newLines))

	for i := range maxLen {
		var oldLine, newLine string

		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine != newLine {
			if oldLine != "" {
				diff.WriteString(fmt.Sprintf("- %s\n", oldLine))
			}
			if newLine != "" {
				diff.WriteString(fmt.Sprintf("+ %s\n", newLine))
			}
		}
	}

	return diff.String()
}

// confirmAction prompts the user for confirmation
func confirmAction(prompt string) bool {
	fmt.Printf("%s (y/N): ", prompt)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
