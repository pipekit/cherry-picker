package github

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
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
	// Pattern for successful cherry-pick: 🍒 Cherry-pick PR created for 3.7: #14944
	successPattern := regexp.MustCompile(`Cherry-pick PR created for ([0-9.]+): #(\d+)`)
	// Pattern for failed cherry-pick: ❌ Cherry-pick failed for 3.7.
	failurePattern := regexp.MustCompile(`Cherry-pick failed for ([0-9.]+)\.`)

	for _, comment := range comments {
		body := comment.GetBody()

		// Check for successful cherry-picks
		matches := successPattern.FindAllStringSubmatch(body, -1)
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
		failMatches := failurePattern.FindAllStringSubmatch(body, -1)
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

	// Pattern to match titles containing "cherry-pick #14894" or "(cherry-pick #14894"
	titlePattern := regexp.MustCompile(fmt.Sprintf(`(?i)cherry-pick\s+#?%d`, prNumber))

	for _, issue := range result.Issues {
		if !issue.IsPullRequest() {
			continue
		}

		// Skip the original PR itself
		if issue.GetNumber() == prNumber {
			continue
		}

		// Check if title matches cherry-pick pattern
		if !titlePattern.MatchString(issue.GetTitle()) {
			continue
		}

		// Get the full PR to determine target branch
		slog.Debug("GitHub API: Getting PR details for manual cherry-pick", "org", c.org, "repo", c.repo, "pr", issue.GetNumber())
		pr, _, err := c.client.PullRequests.Get(ctx, c.org, c.repo, issue.GetNumber())
		if err != nil {
			continue
		}

		// Check if this PR targets one of our tracked branches
		targetBranch := pr.GetBase().GetRef()
		if slices.Contains(branches, targetBranch) {
			cherryPickPRs = append(cherryPickPRs, CherryPickPR{
				Number:     pr.GetNumber(),
				Branch:     targetBranch,
				OriginalPR: prNumber,
				Failed:     false,
			})
		}
	}

	return cherryPickPRs, nil
}
