package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client
type Client struct {
	client *github.Client
	ctx    context.Context
}

// PR represents a pull request from GitHub
type PR struct {
	Number        int
	Title         string
	URL           string
	SHA           string
	Merged        bool
	CIStatus      string   // "passing", "failing", "pending", or "unknown"
	CherryPickFor []string // Target branches extracted from cherry-pick/* labels
}

// Commit represents a commit from GitHub
type Commit struct {
	SHA     string
	Message string
	Author  string
	Date    time.Time
}

// NewClient creates a new GitHub client with token authentication
func NewClient(ctx context.Context, token string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &Client{
		client: github.NewClient(tc),
		ctx:    ctx,
	}
}

// GetMergedPRs fetches merged PRs to the specified branch since the given date
func (c *Client) GetMergedPRs(org, repo, branch string, since time.Time) ([]PR, error) {
	opts := &github.PullRequestListOptions{
		State:     "closed",
		Base:      branch,
		Sort:      "updated",
		Direction: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allPRs []PR

	for {
		prs, resp, err := c.client.PullRequests.List(c.ctx, org, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch pull requests: %w", err)
		}

		for _, pr := range prs {
			// Skip if not merged
			if pr.MergedAt == nil {
				continue
			}

			// Skip if merged before our since date
			if pr.MergedAt.Before(since) {
				// Since we're sorting by updated desc, if we hit an old PR, we can stop
				return allPRs, nil
			}

			// Extract cherry-pick target branches from labels
			cherryPickBranches := extractCherryPickBranches(pr.Labels)

			// Only include PRs that have cherry-pick labels
			if len(cherryPickBranches) == 0 {
				continue
			}

			allPRs = append(allPRs, PR{
				Number:        pr.GetNumber(),
				Title:         pr.GetTitle(),
				URL:           pr.GetHTMLURL(),
				SHA:           pr.GetMergeCommitSHA(),
				Merged:        pr.MergedAt != nil,
				CIStatus:      "unknown", // CI status not fetched for listing
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

// GetOpenPRs fetches open PRs targeting the specified branch
func (c *Client) GetOpenPRs(org, repo, branch string) ([]PR, error) {
	opts := &github.PullRequestListOptions{
		State:     "open",
		Base:      branch,
		Sort:      "updated",
		Direction: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allPRs []PR

	for {
		prs, resp, err := c.client.PullRequests.List(c.ctx, org, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch open pull requests: %w", err)
		}

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

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allPRs, nil
}

// GetPR fetches details for a specific PR by number
func (c *Client) GetPR(org, repo string, number int) (*PR, error) {
	pr, _, err := c.client.PullRequests.Get(c.ctx, org, repo, number)
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
func (c *Client) GetPRWithDetails(org, repo string, number int) (*PR, error) {
	pr, _, err := c.client.PullRequests.Get(c.ctx, org, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR #%d: %w", number, err)
	}

	// Check CI status by getting commit status
	ciStatus, err := c.getPRCIStatus(org, repo, pr.GetHead().GetSHA())
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

// getPRCIStatus checks the CI status of a commit by examining both combined status and check runs
func (c *Client) getPRCIStatus(org, repo, sha string) (string, error) {
	// Get both combined status and check runs for more accurate status
	combinedStatus, checkRunsStatus, err := c.getDetailedCIStatus(org, repo, sha)
	if err != nil {
		return "unknown", fmt.Errorf("failed to fetch CI status for commit %s: %w", sha, err)
	}

	// If either combined status or check runs indicate pending, report as pending
	if combinedStatus == "pending" || checkRunsStatus == "pending" {
		return "pending", nil
	}

	// If either indicates failure, report as failing
	if combinedStatus == "failing" || checkRunsStatus == "failing" {
		return "failing", nil
	}

	// If both are passing, report as passing
	if combinedStatus == "passing" && checkRunsStatus == "passing" {
		return "passing", nil
	}

	// Default to the combined status if we can't determine from check runs
	return combinedStatus, nil
}

// getDetailedCIStatus gets both combined status and check runs status
func (c *Client) getDetailedCIStatus(org, repo, sha string) (string, string, error) {
	// Get combined status (traditional status checks)
	combinedStatus, err := c.getCombinedStatus(org, repo, sha)
	if err != nil {
		return "unknown", "unknown", err
	}

	// Get check runs status (GitHub Actions and other apps)
	checkRunsStatus, err := c.getCheckRunsStatus(org, repo, sha)
	if err != nil {
		// If check runs fail, just use combined status
		return combinedStatus, "unknown", nil
	}

	return combinedStatus, checkRunsStatus, nil
}

// getCombinedStatus gets the traditional combined status, ignoring DCO checks
func (c *Client) getCombinedStatus(org, repo, sha string) (string, error) {
	status, _, err := c.client.Repositories.GetCombinedStatus(c.ctx, org, repo, sha, nil)
	if err != nil {
		return "unknown", err
	}

	// Filter out DCO-related statuses
	var nonDCOStatuses []*github.RepoStatus
	for _, s := range status.Statuses {
		if !isDCOCheck(s.GetContext()) {
			nonDCOStatuses = append(nonDCOStatuses, s)
		}
	}

	// If no non-DCO statuses, return unknown
	if len(nonDCOStatuses) == 0 {
		return "unknown", nil
	}

	// Determine overall status from non-DCO statuses
	hasFailure := false
	hasPending := false
	hasSuccess := false

	for _, s := range nonDCOStatuses {
		switch s.GetState() {
		case "success":
			hasSuccess = true
		case "failure", "error":
			hasFailure = true
		case "pending":
			hasPending = true
		}
	}

	// Priority: pending > failure > success
	if hasPending {
		return "pending", nil
	}
	if hasFailure {
		return "failing", nil
	}
	if hasSuccess {
		return "passing", nil
	}

	return "unknown", nil
}

// getCheckRunsStatus gets the status from GitHub Actions and other check runs
func (c *Client) getCheckRunsStatus(org, repo, sha string) (string, error) {
	checkRuns, _, err := c.client.Checks.ListCheckRunsForRef(c.ctx, org, repo, sha, nil)
	if err != nil {
		return "unknown", err
	}

	if len(checkRuns.CheckRuns) == 0 {
		return "unknown", nil
	}

	hasRunning := false
	hasFailed := false
	hasCompleted := false

	for _, run := range checkRuns.CheckRuns {
		// Skip DCO checks - ignore their status
		if isDCOCheck(run.GetName()) {
			continue
		}

		switch run.GetStatus() {
		case "queued", "in_progress":
			hasRunning = true
		case "completed":
			hasCompleted = true
			if run.GetConclusion() == "failure" || run.GetConclusion() == "cancelled" || run.GetConclusion() == "timed_out" {
				hasFailed = true
			}
		}
	}

	// If any checks are still running, report as pending
	if hasRunning {
		return "pending", nil
	}

	// If any completed checks failed, report as failing
	if hasFailed {
		return "failing", nil
	}

	// If we have completed checks and none failed, report as passing
	if hasCompleted {
		return "passing", nil
	}

	return "unknown", nil
}

// isDCOCheck determines if a check run is a DCO (Developer Certificate of Origin) check
func isDCOCheck(checkName string) bool {
	// Common DCO check names - add more patterns as needed
	dcoPatterns := []string{
		"dco",
		"DCO",
		"developer-certificate-of-origin",
		"signoff",
		"sign-off",
		"signed-off-by",
	}

	for _, pattern := range dcoPatterns {
		if strings.Contains(strings.ToLower(checkName), strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// extractCherryPickBranches extracts target branches from cherry-pick/* labels
// For example, "cherry-pick/3.6" becomes "release-3.6"
func extractCherryPickBranches(labels []*github.Label) []string {
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

// CreatePR creates a new pull request
func (c *Client) CreatePR(org, repo, title, body, head, base string) (int, error) {
	newPR := &github.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &head,
		Base:  &base,
	}

	pr, _, err := c.client.PullRequests.Create(c.ctx, org, repo, newPR)
	if err != nil {
		return 0, err
	}

	return pr.GetNumber(), nil
}

// RetryFailedWorkflows retries all failed workflow runs for a PR
func (c *Client) RetryFailedWorkflows(org, repo string, prNumber int) error {
	// Get the PR to find its head SHA
	pr, _, err := c.client.PullRequests.Get(c.ctx, org, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get PR #%d: %w", prNumber, err)
	}

	headSHA := pr.GetHead().GetSHA()

	// Get workflow runs for the PR's head commit
	workflowRuns, err := c.getWorkflowRunsForCommit(org, repo, headSHA)
	if err != nil {
		return fmt.Errorf("failed to get workflow runs for commit %s: %w", headSHA, err)
	}

	if len(workflowRuns) == 0 {
		return fmt.Errorf("no workflow runs found for PR #%d", prNumber)
	}

	// Retry failed workflow runs
	var retriedCount int
	var errors []error

	for _, run := range workflowRuns {
		// Only retry failed or cancelled runs
		if run.GetConclusion() != "failure" && run.GetConclusion() != "cancelled" && run.GetConclusion() != "timed_out" {
			continue
		}

		err := c.retryWorkflowRun(org, repo, run.GetID())
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to retry workflow run %d: %w", run.GetID(), err))
			continue
		}

		retriedCount++
	}

	if retriedCount == 0 {
		if len(errors) > 0 {
			return fmt.Errorf("no workflow runs were retried due to errors: %v", errors)
		}
		return fmt.Errorf("no failed workflow runs found for PR #%d", prNumber)
	}

	if len(errors) > 0 {
		return fmt.Errorf("retried %d workflow runs but %d failed: %v", retriedCount, len(errors), errors)
	}

	return nil
}

// getWorkflowRunsForCommit gets all workflow runs for a specific commit
func (c *Client) getWorkflowRunsForCommit(org, repo, sha string) ([]*github.WorkflowRun, error) {
	// List workflow runs for the repository, filtered by head_sha
	opts := &github.ListWorkflowRunsOptions{
		HeadSHA: sha,
		ListOptions: github.ListOptions{
			PerPage: 100, // Get up to 100 runs
		},
	}

	runs, _, err := c.client.Actions.ListRepositoryWorkflowRuns(c.ctx, org, repo, opts)
	if err != nil {
		return nil, err
	}

	return runs.WorkflowRuns, nil
}

// retryWorkflowRun retries a specific workflow run by re-running failed jobs
func (c *Client) retryWorkflowRun(org, repo string, runID int64) error {
	// Try to re-run failed jobs first (more targeted approach)
	_, err := c.client.Actions.RerunFailedJobsByID(c.ctx, org, repo, runID)
	if err != nil {
		// If re-running failed jobs doesn't work, try re-running the entire workflow
		_, retryErr := c.client.Actions.RerunWorkflowByID(c.ctx, org, repo, runID)
		if retryErr != nil {
			return fmt.Errorf("failed to retry workflow run (tried both failed jobs and full rerun): %w (original: %v)", retryErr, err)
		}
	}

	return nil
}

// MergePR merges a pull request using the specified merge method
func (c *Client) MergePR(org, repo string, prNumber int, mergeMethod string) error {
	// Get the PR to find its head SHA for merge validation
	pr, _, err := c.client.PullRequests.Get(c.ctx, org, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get PR #%d: %w", prNumber, err)
	}

	// Check if PR is mergeable (can be nil, true, or false)
	if pr.Mergeable != nil && !*pr.Mergeable {
		return fmt.Errorf("PR #%d is not mergeable (conflicts may exist)", prNumber)
	}

	// Prepare merge options with squash method
	commitTitle := fmt.Sprintf("%s (#%d)", pr.GetTitle(), prNumber)
	mergeOptions := &github.PullRequestOptions{
		CommitTitle: commitTitle,
		MergeMethod: mergeMethod,
	}

	// Perform the merge
	mergeResult, _, err := c.client.PullRequests.Merge(c.ctx, org, repo, prNumber, "", mergeOptions)
	if err != nil {
		return fmt.Errorf("failed to merge PR #%d: %w", prNumber, err)
	}

	if !mergeResult.GetMerged() {
		return fmt.Errorf("PR #%d merge was not successful: %s", prNumber, mergeResult.GetMessage())
	}

	return nil
}

// ListTags gets all tags from the repository
func (c *Client) ListTags(org, repo string) ([]string, error) {
	opts := &github.ListOptions{
		PerPage: 100,
	}

	var allTags []string

	for {
		tags, resp, err := c.client.Repositories.ListTags(c.ctx, org, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list tags: %w", err)
		}

		for _, tag := range tags {
			allTags = append(allTags, tag.GetName())
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allTags, nil
}

// GetCommitsSince gets commits on a branch since a specific tag/commit (equivalent to git log tag..branch)
func (c *Client) GetCommitsSince(org, repo, branch, sinceTag string) ([]Commit, error) {
	var base string

	if sinceTag == "v0.0.0" {
		// Special case for initial version - get all commits on the branch
		// We'll use an empty base which means "from the beginning"
		base = ""
	} else {
		// Use the tag as the base for comparison
		base = sinceTag
	}

	// Use GitHub's Compare API to get commits between base and head
	// This is equivalent to "git log base..head"
	var comparison *github.CommitsComparison
	var err error

	if base == "" {
		// For initial version, get all commits on the branch
		opts := &github.CommitsListOptions{
			SHA: branch,
			ListOptions: github.ListOptions{
				PerPage: 100,
			},
		}

		var allCommits []Commit
		for {
			commits, resp, err := c.client.Repositories.ListCommits(c.ctx, org, repo, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list commits: %w", err)
			}

			for _, commit := range commits {
				allCommits = append(allCommits, Commit{
					SHA:     commit.GetSHA(),
					Message: strings.Split(commit.GetCommit().GetMessage(), "\n")[0], // First line only
					Author:  commit.GetCommit().GetAuthor().GetName(),
					Date:    commit.GetCommit().GetAuthor().GetDate().Time,
				})
			}

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}
		return allCommits, nil
	} else {
		// Use Compare API for tag..branch comparison
		comparison, _, err = c.client.Repositories.CompareCommits(c.ctx, org, repo, base, branch, &github.ListOptions{
			PerPage: 100,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to compare %s..%s: %w", base, branch, err)
		}
	}

	// Convert GitHub commits to our Commit struct
	var commits []Commit
	for _, commit := range comparison.Commits {
		commits = append(commits, Commit{
			SHA:     commit.GetSHA(),
			Message: strings.Split(commit.GetCommit().GetMessage(), "\n")[0], // First line only
			Author:  commit.GetCommit().GetAuthor().GetName(),
			Date:    commit.GetCommit().GetAuthor().GetDate().Time,
		})
	}

	return commits, nil
}
