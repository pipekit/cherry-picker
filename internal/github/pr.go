package github

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
)

// GetMergedPRs fetches all merged PRs to the specified branch with cherry-pick labels
func (c *Client) GetMergedPRs(branch string, since time.Time) ([]PR, error) {
	labels, err := c.ListLabels()
	if err != nil {
		return nil, fmt.Errorf("failed to list labels: %w", err)
	}

	cherryPickLabels := filterCherryPickLabels(labels)
	if len(cherryPickLabels) == 0 {
		return []PR{}, nil
	}

	query := buildSearchQuery(c.org, c.repo, branch, cherryPickLabels)
	return c.searchPRs(query)
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

	for _, label := range labels {
		parts = append(parts, fmt.Sprintf("label:\"%s\"", label))
	}

	return strings.Join(parts, " ")
}

// searchPRs executes a search query and returns matching PRs
func (c *Client) searchPRs(query string) ([]PR, error) {
	opts := &github.SearchOptions{
		Sort:  "updated",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allPRs []PR

	for {
		result, resp, err := c.client.Search.Issues(c.ctx, query, opts)
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
				pr, _, err := c.client.PullRequests.Get(c.ctx, extractOrgFromIssue(issue), extractRepoFromIssue(issue), prNum)
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
		if strings.HasPrefix(labelName, "cherry-pick/") {
			version := strings.TrimPrefix(labelName, "cherry-pick/")
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
func (c *Client) GetOpenPRs(branch string) ([]PR, error) {
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
		return c.client.PullRequests.List(c.ctx, c.org, c.repo, opts)
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
func (c *Client) GetPR(number int) (*PR, error) {
	pr, _, err := c.client.PullRequests.Get(c.ctx, c.org, c.repo, number)
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
func (c *Client) GetPRWithDetails(number int) (*PR, error) {
	pr, _, err := c.client.PullRequests.Get(c.ctx, c.org, c.repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR #%d: %w", number, err)
	}

	// Check CI status by getting commit status
	ciStatus, err := c.getPRCIStatus(pr.GetHead().GetSHA())
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
func (c *Client) getPRCIStatus(sha string) (string, error) {
	checker := c.newCIStatusChecker()
	return checker.GetStatus(sha)
}

// CreatePR creates a new pull request
func (c *Client) CreatePR(title, body, head, base string) (int, error) {
	newPR := &github.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &head,
		Base:  &base,
	}

	pr, _, err := c.client.PullRequests.Create(c.ctx, c.org, c.repo, newPR)
	if err != nil {
		return 0, err
	}

	return pr.GetNumber(), nil
}
