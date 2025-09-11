package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/github"
)

// BaseCommand provides common fields and initialization for all commands
type BaseCommand struct {
	ConfigFile   *string
	LoadConfig   func(string) (*cmd.Config, error)
	SaveConfig   func(string, *cmd.Config) error
	GitHubClient *github.Client
	Context      context.Context
	Config       *cmd.Config
}

// Init initializes the base command with common setup
func (bc *BaseCommand) Init() error {
	// Load configuration
	config, err := bc.LoadConfig(*bc.ConfigFile)
	if err != nil {
		return err
	}
	bc.Config = config

	// Initialize GitHub client with token from environment
	token, err := getGitHubToken()
	if err != nil {
		return err
	}
	bc.Context = context.Background()
	bc.GitHubClient = github.NewClient(bc.Context, token)

	return nil
}

// getGitHubToken retrieves and validates the GitHub token
func getGitHubToken() (string, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return "", fmt.Errorf("GITHUB_TOKEN environment variable is required")
	}
	return token, nil
}

// SaveConfigWithErrorHandling saves the config with standardized error handling
func (bc *BaseCommand) SaveConfigWithErrorHandling(config *cmd.Config) error {
	if err := bc.SaveConfig(*bc.ConfigFile, config); err != nil {
		return err
	}
	return nil
}
