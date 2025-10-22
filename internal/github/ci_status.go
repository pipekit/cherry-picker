package github

import (
	"fmt"
	"strings"

	"github.com/google/go-github/v57/github"
)

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
