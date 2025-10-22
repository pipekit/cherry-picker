package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/github"
)

// InitializeGitHubClient creates a GitHub client with proper token validation and repository context
func InitializeGitHubClient(config *cmd.Config) (*github.Client, context.Context, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, nil, fmt.Errorf("GITHUB_TOKEN environment variable is required")
	}

	ctx := context.Background()
	client := github.NewClient(ctx, token).WithRepository(config.Org, config.Repo)

	return client, ctx, nil
}
