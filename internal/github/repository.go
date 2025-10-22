package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v57/github"
)

// ListTags gets all tags from the repository
func (c *Client) ListTags(ctx context.Context) ([]string, error) {
	tags, err := paginatedList(func(page int) ([]*github.RepositoryTag, *github.Response, error) {
		opts := &github.ListOptions{
			PerPage: 100,
			Page:    page,
		}
		return c.client.Repositories.ListTags(ctx, c.org, c.repo, opts)
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
func (c *Client) ListLabels(ctx context.Context) ([]*github.Label, error) {
	labels, err := paginatedList(func(page int) ([]*github.Label, *github.Response, error) {
		opts := &github.ListOptions{
			PerPage: 100,
			Page:    page,
		}
		return c.client.Issues.ListLabels(ctx, c.org, c.repo, opts)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list labels: %w", err)
	}

	return labels, nil
}

// GetCommitsSince gets commits on a branch since a specific tag/commit (equivalent to git log tag..branch)
func (c *Client) GetCommitsSince(ctx context.Context, branch, sinceTag string) ([]Commit, error) {
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
			return c.client.Repositories.ListCommits(ctx, c.org, c.repo, opts)
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
		comparison, _, err = c.client.Repositories.CompareCommits(ctx, c.org, c.repo, base, branch, &github.ListOptions{
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
