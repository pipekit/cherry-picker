package github

import (
	"context"

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
