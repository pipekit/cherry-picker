package github

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
)

// GetMergedPRs fetches all merged PRs to the specified branch with cherry-pick labels
// Note: The _since parameter is kept for potential future use but is not currently used in queries
func (c *Client) GetMergedPRs(ctx context.Context, branch string, _since time.Time) ([]PR, error) {
	labels, err := c.ListLabels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list labels: %w", err)
	}

	cherryPickLabels := filterCherryPickLabels(labels)
	if len(cherryPickLabels) == 0 {
		return []PR{}, nil
	}

	query := buildSearchQuery(c.org, c.repo, branch, cherryPickLabels)
	return c.searchPRs(ctx, query)
}

// filterCherryPickLabels filters labels to only those starting with "cherry-pick"
func filterCherryPickLabels(labels []*github.Label) []string {
	var cherryPickLabels []string
	for _, label := range labels {
		if strings.HasPrefix(label.GetName(), "cherry-pick") {
			cherryPickLabels = append(cherryPickLabels, label.GetName())
		}
	}
	return cherryPickLabels
}

// buildSearchQuery constructs a GitHub search query for merged PRs with cherry-pick labels
func buildSearchQuery(org, repo, branch string, labels []string) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("repo:%s/%s", org, repo))
	parts = append(parts, "is:pr")
	parts = append(parts, "is:merged")
	parts = append(parts, fmt.Sprintf("base:%s", branch))

	// Use OR logic for labels so PRs with any cherry-pick label are included
	if len(labels) > 0 {
		var labelParts []string
		for _, label := range labels {
			labelParts = append(labelParts, fmt.Sprintf("label:\"%s\"", label))
		}
		parts = append(parts, "("+strings.Join(labelParts, " OR ")+")")
	}

	return strings.Join(parts, " ")
}

// searchPRs executes a search query and returns matching PRs
func (c *Client) searchPRs(ctx context.Context, query string) ([]PR, error) {
	opts := &github.SearchOptions{
		Sort:  "updated",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allPRs []PR

	for {
		slog.Debug("GitHub API: Searching issues/PRs", "query", query, "page", opts.Page)
		result, resp, err := c.client.Search.Issues(ctx, query, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to search PRs: %w", err)
		}

		for _, issue := range result.Issues {
			if !issue.IsPullRequest() {
				continue
			}

			cherryPickBranches := extractCherryPickBranchesFromLabels(issue.Labels)
			if len(cherryPickBranches) == 0 {
				continue
			}

			var sha string
			if issue.PullRequestLinks != nil && issue.PullRequestLinks.URL != nil {
				prNum := issue.GetNumber()
				slog.Debug("GitHub API: Getting PR details", "org", extractOrgFromIssue(issue), "repo", extractRepoFromIssue(issue), "pr", prNum)
				pr, _, err := c.client.PullRequests.Get(ctx, extractOrgFromIssue(issue), extractRepoFromIssue(issue), prNum)
				if err == nil && pr.MergeCommitSHA != nil {
					sha = pr.GetMergeCommitSHA()
				}
			}

			allPRs = append(allPRs, PR{
				Number:        issue.GetNumber(),
				Title:         issue.GetTitle(),
				URL:           issue.GetHTMLURL(),
				SHA:           sha,
				Merged:        issue.ClosedAt != nil,
				CIStatus:      "unknown",
				CherryPickFor: cherryPickBranches,
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allPRs, nil
}

// extractCherryPickBranchesFromLabels extracts target branches from cherry-pick/* labels
// For example, "cherry-pick/3.6" becomes "release-3.6"
func extractCherryPickBranchesFromLabels(labels []*github.Label) []string {
	var branches []string
	for _, label := range labels {
		labelName := label.GetName()
		if after, ok := strings.CutPrefix(labelName, "cherry-pick/"); ok {
			version := after
			branch := "release-" + version
			branches = append(branches, branch)
		}
	}
	return branches
}

// extractOrgFromIssue extracts org from issue repository URL
func extractOrgFromIssue(issue *github.Issue) string {
	if issue.Repository == nil {
		return ""
	}
	if issue.Repository.Owner == nil {
		return ""
	}
	return issue.Repository.Owner.GetLogin()
}

// extractRepoFromIssue extracts repo name from issue
func extractRepoFromIssue(issue *github.Issue) string {
	if issue.Repository == nil {
		return ""
	}
	return issue.Repository.GetName()
}

// GetOpenPRs fetches open PRs targeting the specified branch
func (c *Client) GetOpenPRs(ctx context.Context, branch string) ([]PR, error) {
	prs, err := paginatedList(func(page int) ([]*github.PullRequest, *github.Response, error) {
		opts := &github.PullRequestListOptions{
			State:     "open",
			Base:      branch,
			Sort:      "updated",
			Direction: "desc",
			ListOptions: github.ListOptions{
				PerPage: 100,
				Page:    page,
			},
		}
		slog.Debug("GitHub API: Listing pull requests", "org", c.org, "repo", c.repo, "base", branch, "state", "open", "page", page)
		return c.client.PullRequests.List(ctx, c.org, c.repo, opts)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch open pull requests: %w", err)
	}

	// Convert to our PR type
	var allPRs []PR
	for _, pr := range prs {
		allPRs = append(allPRs, PR{
			Number:   pr.GetNumber(),
			Title:    pr.GetTitle(),
			URL:      pr.GetHTMLURL(),
			SHA:      pr.GetHead().GetSHA(), // Use head SHA for open PRs
			Merged:   false,                 // Open PRs are not merged
			CIStatus: "unknown",             // CI status not fetched for listing
		})
	}

	return allPRs, nil
}

// GetPR fetches details for a specific PR by number
func (c *Client) GetPR(ctx context.Context, number int) (*PR, error) {
	slog.Debug("GitHub API: Getting PR", "org", c.org, "repo", c.repo, "pr", number)
	pr, _, err := c.client.PullRequests.Get(ctx, c.org, c.repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR #%d: %w", number, err)
	}

	return &PR{
		Number:   pr.GetNumber(),
		Title:    pr.GetTitle(),
		URL:      pr.GetHTMLURL(),
		SHA:      pr.GetMergeCommitSHA(),
		Merged:   pr.MergedAt != nil,
		CIStatus: "unknown", // CI status not fetched in simple PR fetch
	}, nil
}

// GetPRWithDetails fetches detailed information for a specific PR including CI status
func (c *Client) GetPRWithDetails(ctx context.Context, number int) (*PR, error) {
	slog.Debug("GitHub API: Getting PR with details", "org", c.org, "repo", c.repo, "pr", number)
	pr, _, err := c.client.PullRequests.Get(ctx, c.org, c.repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR #%d: %w", number, err)
	}

	// Check CI status by getting commit status
	ciStatus, err := c.getPRCIStatus(ctx, pr.GetHead().GetSHA())
	if err != nil {
		// Don't fail the whole request if we can't get CI status
		ciStatus = "unknown"
	}

	return &PR{
		Number:   pr.GetNumber(),
		Title:    pr.GetTitle(),
		URL:      pr.GetHTMLURL(),
		SHA:      pr.GetMergeCommitSHA(),
		Merged:   pr.MergedAt != nil,
		CIStatus: ciStatus,
	}, nil
}

// getPRCIStatus checks the CI status of a commit using the CIStatusChecker
func (c *Client) getPRCIStatus(ctx context.Context, sha string) (string, error) {
	checker := c.newCIStatusChecker()
	return checker.GetStatus(ctx, sha)
}

// CreatePR creates a new pull request
func (c *Client) CreatePR(ctx context.Context, title, body, head, base string) (int, error) {
	newPR := &github.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &head,
		Base:  &base,
	}

	slog.Debug("GitHub API: Creating PR", "org", c.org, "repo", c.repo, "head", head, "base", base)
	pr, _, err := c.client.PullRequests.Create(ctx, c.org, c.repo, newPR)
	if err != nil {
		return 0, err
	}

	return pr.GetNumber(), nil
}
