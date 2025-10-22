package pick

import (
	"fmt"
	"strings"
)

// CherryPickResult holds the result of a cherry-pick operation
type CherryPickResult struct {
	PRNumber int
	Title    string
	CIStatus string
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
