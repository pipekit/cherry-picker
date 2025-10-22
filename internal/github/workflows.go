package github

import (
	"fmt"

	"github.com/google/go-github/v57/github"
)

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
