package github

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/go-github/v57/github"
)

// GetIssueComments retrieves all comments for a specific issue
func (c *Client) GetIssueComments(ctx context.Context, issueNumber int) ([]Comment, error) {
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allComments []Comment
	for {
		slog.Debug("GitHub API: Listing issue comments", "org", c.org, "repo", c.repo, "issue", issueNumber, "page", opts.Page)
		comments, resp, err := c.client.Issues.ListComments(ctx, c.org, c.repo, issueNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list issue comments: %w", err)
		}

		for _, comment := range comments {
			allComments = append(allComments, Comment{
				ID:        comment.GetID(),
				Body:      comment.GetBody(),
				User:      comment.GetUser().GetLogin(),
				CreatedAt: comment.GetCreatedAt().Time,
				UpdatedAt: comment.GetUpdatedAt().Time,
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
}

// CreateIssueComment creates a new comment on an issue
func (c *Client) CreateIssueComment(ctx context.Context, issueNumber int, body string) (*Comment, error) {
	commentInput := &github.IssueComment{
		Body: github.String(body),
	}

	slog.Debug("GitHub API: Creating issue comment", "org", c.org, "repo", c.repo, "issue", issueNumber)
	comment, _, err := c.client.Issues.CreateComment(ctx, c.org, c.repo, issueNumber, commentInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	return &Comment{
		ID:        comment.GetID(),
		Body:      comment.GetBody(),
		User:      comment.GetUser().GetLogin(),
		CreatedAt: comment.GetCreatedAt().Time,
		UpdatedAt: comment.GetUpdatedAt().Time,
	}, nil
}

// UpdateIssueComment updates an existing comment
func (c *Client) UpdateIssueComment(ctx context.Context, commentID int64, body string) (*Comment, error) {
	commentInput := &github.IssueComment{
		Body: github.String(body),
	}

	slog.Debug("GitHub API: Updating issue comment", "org", c.org, "repo", c.repo, "comment_id", commentID)
	comment, _, err := c.client.Issues.EditComment(ctx, c.org, c.repo, commentID, commentInput)
	if err != nil {
		return nil, fmt.Errorf("failed to update comment: %w", err)
	}

	return &Comment{
		ID:        comment.GetID(),
		Body:      comment.GetBody(),
		User:      comment.GetUser().GetLogin(),
		CreatedAt: comment.GetCreatedAt().Time,
		UpdatedAt: comment.GetUpdatedAt().Time,
	}, nil
}

// GetAuthenticatedUser returns the login name of the authenticated user
func (c *Client) GetAuthenticatedUser(ctx context.Context) (string, error) {
	slog.Debug("GitHub API: Getting authenticated user")
	user, _, err := c.client.Users.Get(ctx, "")
	if err != nil {
		return "", fmt.Errorf("failed to get authenticated user: %w", err)
	}

	return user.GetLogin(), nil
}
