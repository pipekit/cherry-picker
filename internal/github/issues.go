package github

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/go-github/v57/github"
)

// SearchIssuesByText searches for open issues containing the specified text
func (c *Client) SearchIssuesByText(ctx context.Context, searchText string) ([]Issue, error) {
	query := fmt.Sprintf("repo:%s/%s is:issue is:open \"%s\" in:title,body", c.org, c.repo, searchText)

	opts := &github.SearchOptions{
		Sort:  "updated",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allIssues []Issue

	for {
		slog.Debug("GitHub API: Searching issues", "query", query, "page", opts.Page)
		result, resp, err := c.client.Search.Issues(ctx, query, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to search issues: %w", err)
		}

		for _, issue := range result.Issues {
			// Skip pull requests
			if issue.IsPullRequest() {
				continue
			}

			allIssues = append(allIssues, Issue{
				Number: issue.GetNumber(),
				Title:  issue.GetTitle(),
				Body:   issue.GetBody(),
				URL:    issue.GetHTMLURL(),
				State:  issue.GetState(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allIssues, nil
}
