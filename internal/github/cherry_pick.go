package github

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/google/go-github/v57/github"
)

// GetCherryPickPRsFromComments extracts cherry-pick PR numbers and failures from bot comments
// Looks for patterns like:
//   - Success: "🍒 Cherry-pick PR created for 3.7: #14944"
//   - Failure: "Cherry-pick failed for 3.7" or similar failure messages
func (c *Client) GetCherryPickPRsFromComments(ctx context.Context, prNumber int) ([]CherryPickPR, error) {
	slog.Debug("GitHub API: Listing comments", "org", c.org, "repo", c.repo, "pr", prNumber)
	comments, _, err := c.client.Issues.ListComments(ctx, c.org, c.repo, prNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch comments for PR #%d: %w", prNumber, err)
	}

	var cherryPickPRs []CherryPickPR

	for _, comment := range comments {
		body := comment.GetBody()

		// Check for successful cherry-picks
		matches := BotSuccessPattern.FindAllStringSubmatch(body, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				version := match[1]
				prNum := match[2]

				// Convert PR number string to int
				var cherryPickNum int
				if _, err := fmt.Sscanf(prNum, "%d", &cherryPickNum); err != nil {
					// Skip if PR number parsing fails
					continue
				}

				cherryPickPRs = append(cherryPickPRs, CherryPickPR{
					Number:     cherryPickNum,
					Branch:     "release-" + version,
					OriginalPR: prNumber,
					Failed:     false,
				})
			}
		}

		// Check for failed cherry-picks
		failMatches := BotFailurePattern.FindAllStringSubmatch(body, -1)
		for _, match := range failMatches {
			if len(match) >= 2 {
				version := match[1]
				cherryPickPRs = append(cherryPickPRs, CherryPickPR{
					Number:     0, // No PR number for failures
					Branch:     "release-" + version,
					OriginalPR: prNumber,
					Failed:     true,
				})
			}
		}
	}

	return cherryPickPRs, nil
}

// SearchManualCherryPickPRs searches for manually created cherry-pick PRs by title pattern
// Looks for PRs with titles like "cherry-pick: ... (#14894)" targeting release branches
func (c *Client) SearchManualCherryPickPRs(ctx context.Context, prNumber int, branches []string) ([]CherryPickPR, error) {
	var cherryPickPRs []CherryPickPR

	// Search for PRs containing "cherry-pick" and the PR number in title
	query := fmt.Sprintf("repo:%s/%s is:pr cherry-pick %d in:title", c.org, c.repo, prNumber)

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	slog.Debug("GitHub API: Searching for manual cherry-pick PRs", "org", c.org, "repo", c.repo, "pr", prNumber, "query", query)
	result, _, err := c.client.Search.Issues(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search for manual cherry-pick PRs: %w", err)
	}

	for _, issue := range result.Issues {
		if !issue.IsPullRequest() {
			continue
		}

		// Skip the original PR itself
		if issue.GetNumber() == prNumber {
			continue
		}

		// Check if title contains a cherry-pick reference for this PR
		title := issue.GetTitle()
		if !ContainsCherryPickForPR(title, prNumber) {
			slog.Debug("Title does not match cherry-pick pattern", "pr", issue.GetNumber(), "title", title)
			continue
		}

		// Try to extract branch from title first (e.g., "cherry-pick #14894 for 3.7")
		var targetBranch string
		if extractedBranch, found := ExtractBranchFromCherryPickTitle(title, prNumber); found {
			if slices.Contains(branches, extractedBranch) {
				targetBranch = extractedBranch
				slog.Debug("Extracted branch from title", "pr", issue.GetNumber(), "branch", targetBranch)
			}
		}

		// If branch not extracted from title, get the full PR to determine target branch
		if targetBranch == "" {
			slog.Debug("GitHub API: Getting PR details for manual cherry-pick", "org", c.org, "repo", c.repo, "pr", issue.GetNumber())
			pr, _, err := c.client.PullRequests.Get(ctx, c.org, c.repo, issue.GetNumber())
			if err != nil {
				slog.Debug("Failed to get PR details", "pr", issue.GetNumber(), "error", err)
				continue
			}
			targetBranch = pr.GetBase().GetRef()
		}

		// Check if this PR targets one of our tracked branches
		if slices.Contains(branches, targetBranch) {
			slog.Debug("Found manual cherry-pick PR", "pr", issue.GetNumber(), "branch", targetBranch, "original", prNumber)
			cherryPickPRs = append(cherryPickPRs, CherryPickPR{
				Number:     issue.GetNumber(),
				Branch:     targetBranch,
				OriginalPR: prNumber,
				Failed:     false,
			})
		} else {
			slog.Debug("Manual cherry-pick PR targets different branch", "pr", issue.GetNumber(), "branch", targetBranch, "tracked", branches)
		}
	}

	return cherryPickPRs, nil
}
