package github

import (
	"fmt"
	"regexp"

	"github.com/google/go-github/v57/github"
)

// GetCherryPickPRsFromComments extracts cherry-pick PR numbers and failures from bot comments
// Looks for patterns like:
//   - Success: "🍒 Cherry-pick PR created for 3.7: #14944"
//   - Failure: "Cherry-pick failed for 3.7" or similar failure messages
func (c *Client) GetCherryPickPRsFromComments(prNumber int) ([]CherryPickPR, error) {
	comments, _, err := c.client.Issues.ListComments(c.ctx, c.org, c.repo, prNumber, nil)
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
				fmt.Sscanf(prNum, "%d", &cherryPickNum)

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
func (c *Client) SearchManualCherryPickPRs(prNumber int, branches []string) ([]CherryPickPR, error) {
	var cherryPickPRs []CherryPickPR

	// Search for PRs containing "cherry-pick" and the PR number in title
	query := fmt.Sprintf("repo:%s/%s is:pr cherry-pick %d in:title", c.org, c.repo, prNumber)

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	result, _, err := c.client.Search.Issues(c.ctx, query, opts)
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
		pr, _, err := c.client.PullRequests.Get(c.ctx, c.org, c.repo, issue.GetNumber())
		if err != nil {
			continue
		}

		// Check if this PR targets one of our tracked branches
		targetBranch := pr.GetBase().GetRef()
		for _, branch := range branches {
			if targetBranch == branch {
				cherryPickPRs = append(cherryPickPRs, CherryPickPR{
					Number:     pr.GetNumber(),
					Branch:     targetBranch,
					OriginalPR: prNumber,
					Failed:     false,
				})
				break
			}
		}
	}

	return cherryPickPRs, nil
}
