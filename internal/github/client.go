package github

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client with repository context
type Client struct {
	client *github.Client
	ctx    context.Context
	org    string
	repo   string
}

// paginatedList handles paginated list operations
// fetchPage should return the items for the current page and the response with pagination info
func paginatedList[T any](fetchPage func(page int) ([]T, *github.Response, error)) ([]T, error) {
	var allItems []T
	page := 0

	for {
		items, resp, err := fetchPage(page)
		if err != nil {
			return nil, err
		}

		allItems = append(allItems, items...)

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	return allItems, nil
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

// WithRepository returns a new client with org/repo context set
func (c *Client) WithRepository(org, repo string) *Client {
	return &Client{
		client: c.client,
		ctx:    c.ctx,
		org:    org,
		repo:   repo,
	}
}

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

// CIStatusChecker handles checking CI status for commits, filtering out DCO checks
type CIStatusChecker struct {
	client      *Client
	dcoPatterns []string
}

// newCIStatusChecker creates a new CI status checker with default DCO patterns
func (c *Client) newCIStatusChecker() *CIStatusChecker {
	return &CIStatusChecker{
		client: c,
		dcoPatterns: []string{
			"dco",
			"DCO",
			"developer-certificate-of-origin",
			"signoff",
			"sign-off",
			"signed-off-by",
		},
	}
}

// isDCOCheck determines if a check name matches DCO patterns
func (checker *CIStatusChecker) isDCOCheck(checkName string) bool {
	lowerName := strings.ToLower(checkName)
	for _, pattern := range checker.dcoPatterns {
		if strings.Contains(lowerName, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// GetStatus returns the overall CI status for a commit SHA
func (checker *CIStatusChecker) GetStatus(sha string) (string, error) {
	// Get both combined status and check runs for more accurate status
	combinedStatus, checkRunsStatus, err := checker.getDetailedStatus(sha)
	if err != nil {
		return "unknown", fmt.Errorf("failed to fetch CI status for commit %s: %w", sha, err)
	}

	return checker.aggregateStatus(combinedStatus, checkRunsStatus), nil
}

// aggregateStatus combines combined status and check runs status with priority rules
func (checker *CIStatusChecker) aggregateStatus(combinedStatus, checkRunsStatus string) string {
	// Priority: pending > failing > passing
	if combinedStatus == "pending" || checkRunsStatus == "pending" {
		return "pending"
	}

	if combinedStatus == "failing" || checkRunsStatus == "failing" {
		return "failing"
	}

	if combinedStatus == "passing" && checkRunsStatus == "passing" {
		return "passing"
	}

	// Default to combined status if inconclusive
	return combinedStatus
}

// getDetailedStatus fetches both traditional status checks and modern check runs
func (checker *CIStatusChecker) getDetailedStatus(sha string) (string, string, error) {
	combinedStatus, err := checker.getCombinedStatus(sha)
	if err != nil {
		return "unknown", "unknown", err
	}

	checkRunsStatus, err := checker.getCheckRunsStatus(sha)
	if err != nil {
		// If check runs fail, just use combined status
		return combinedStatus, "unknown", nil
	}

	return combinedStatus, checkRunsStatus, nil
}

// getCombinedStatus gets traditional commit status, filtering DCO checks
func (checker *CIStatusChecker) getCombinedStatus(sha string) (string, error) {
	status, _, err := checker.client.client.Repositories.GetCombinedStatus(checker.client.ctx, checker.client.org, checker.client.repo, sha, nil)
	if err != nil {
		return "unknown", err
	}

	// Filter out DCO-related statuses
	var relevantStatuses []*github.RepoStatus
	for _, s := range status.Statuses {
		if !checker.isDCOCheck(s.GetContext()) {
			relevantStatuses = append(relevantStatuses, s)
		}
	}

	if len(relevantStatuses) == 0 {
		return "unknown", nil
	}

	return checker.evaluateStatuses(relevantStatuses), nil
}

// evaluateStatuses determines overall status from a list of status checks
func (checker *CIStatusChecker) evaluateStatuses(statuses []*github.RepoStatus) string {
	hasFailure := false
	hasPending := false
	hasSuccess := false

	for _, s := range statuses {
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
		return "pending"
	}
	if hasFailure {
		return "failing"
	}
	if hasSuccess {
		return "passing"
	}

	return "unknown"
}

// getCheckRunsStatus gets status from GitHub Actions and modern check runs
func (checker *CIStatusChecker) getCheckRunsStatus(sha string) (string, error) {
	checkRuns, _, err := checker.client.client.Checks.ListCheckRunsForRef(checker.client.ctx, checker.client.org, checker.client.repo, sha, nil)
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
		// Skip DCO checks
		if checker.isDCOCheck(run.GetName()) {
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

	// Priority: running > failed > completed
	if hasRunning {
		return "pending", nil
	}
	if hasFailed {
		return "failing", nil
	}
	if hasCompleted {
		return "passing", nil
	}

	return "unknown", nil
}

// CherryPickPR represents a cherry-pick PR created by a bot
type CherryPickPR struct {
	Number     int
	Branch     string
	OriginalPR int
	Failed     bool // True if cherry-pick attempt failed
}

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

// RetryFailedWorkflows retries all failed workflow runs for a PR
func (c *Client) RetryFailedWorkflows(prNumber int) error {
	// Get the PR to find its head SHA
	pr, _, err := c.client.PullRequests.Get(c.ctx, c.org, c.repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get PR #%d: %w", prNumber, err)
	}

	headSHA := pr.GetHead().GetSHA()

	// Get workflow runs for the PR's head commit
	workflowRuns, err := c.getWorkflowRunsForCommit(headSHA)
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

		err := c.retryWorkflowRun(run.GetID())
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
func (c *Client) getWorkflowRunsForCommit(sha string) ([]*github.WorkflowRun, error) {
	// List workflow runs for the repository, filtered by head_sha
	opts := &github.ListWorkflowRunsOptions{
		HeadSHA: sha,
		ListOptions: github.ListOptions{
			PerPage: 100, // Get up to 100 runs
		},
	}

	runs, _, err := c.client.Actions.ListRepositoryWorkflowRuns(c.ctx, c.org, c.repo, opts)
	if err != nil {
		return nil, err
	}

	return runs.WorkflowRuns, nil
}

// retryWorkflowRun retries a specific workflow run by re-running failed jobs
func (c *Client) retryWorkflowRun(runID int64) error {
	// Try to re-run failed jobs first (more targeted approach)
	_, err := c.client.Actions.RerunFailedJobsByID(c.ctx, c.org, c.repo, runID)
	if err != nil {
		// If re-running failed jobs doesn't work, try re-running the entire workflow
		_, retryErr := c.client.Actions.RerunWorkflowByID(c.ctx, c.org, c.repo, runID)
		if retryErr != nil {
			return fmt.Errorf("failed to retry workflow run (tried both failed jobs and full rerun): %w (original: %v)", retryErr, err)
		}
	}

	return nil
}

// MergePR merges a pull request using the specified merge method
func (c *Client) MergePR(prNumber int, mergeMethod string) error {
	// Get the PR to find its head SHA for merge validation
	pr, _, err := c.client.PullRequests.Get(c.ctx, c.org, c.repo, prNumber)
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
	mergeResult, _, err := c.client.PullRequests.Merge(c.ctx, c.org, c.repo, prNumber, "", mergeOptions)
	if err != nil {
		return fmt.Errorf("failed to merge PR #%d: %w", prNumber, err)
	}

	if !mergeResult.GetMerged() {
		return fmt.Errorf("PR #%d merge was not successful: %s", prNumber, mergeResult.GetMessage())
	}

	return nil
}

// ListTags gets all tags from the repository
func (c *Client) ListTags() ([]string, error) {
	tags, err := paginatedList(func(page int) ([]*github.RepositoryTag, *github.Response, error) {
		opts := &github.ListOptions{
			PerPage: 100,
			Page:    page,
		}
		return c.client.Repositories.ListTags(c.ctx, c.org, c.repo, opts)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	// Convert to string slice
	var tagNames []string
	for _, tag := range tags {
		tagNames = append(tagNames, tag.GetName())
	}

	return tagNames, nil
}

// ListLabels fetches all labels from the repository
func (c *Client) ListLabels() ([]*github.Label, error) {
	labels, err := paginatedList(func(page int) ([]*github.Label, *github.Response, error) {
		opts := &github.ListOptions{
			PerPage: 100,
			Page:    page,
		}
		return c.client.Issues.ListLabels(c.ctx, c.org, c.repo, opts)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list labels: %w", err)
	}

	return labels, nil
}

// GetCommitsSince gets commits on a branch since a specific tag/commit (equivalent to git log tag..branch)
func (c *Client) GetCommitsSince(branch, sinceTag string) ([]Commit, error) {
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
		commits, err := paginatedList(func(page int) ([]*github.RepositoryCommit, *github.Response, error) {
			opts := &github.CommitsListOptions{
				SHA: branch,
				ListOptions: github.ListOptions{
					PerPage: 100,
					Page:    page,
				},
			}
			return c.client.Repositories.ListCommits(c.ctx, c.org, c.repo, opts)
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list commits: %w", err)
		}

		// Convert to our Commit type
		var allCommits []Commit
		for _, commit := range commits {
			allCommits = append(allCommits, Commit{
				SHA:     commit.GetSHA(),
				Message: strings.Split(commit.GetCommit().GetMessage(), "\n")[0], // First line only
				Author:  commit.GetCommit().GetAuthor().GetName(),
				Date:    commit.GetCommit().GetAuthor().GetDate().Time,
			})
		}
		return allCommits, nil
	} else {
		// Use Compare API for tag..branch comparison
		comparison, _, err = c.client.Repositories.CompareCommits(c.ctx, c.org, c.repo, base, branch, &github.ListOptions{
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
