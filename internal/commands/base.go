package commands

import (
	"context"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/github"
)

// BaseCommand provides common fields and initialization for all commands
type BaseCommand struct {
	ConfigFile   *string
	LoadConfig   func(string) (*cmd.Config, error)
	SaveConfig   func(string, *cmd.Config) error
	GitHubClient *github.Client
	Config       *cmd.Config
}

// Init initializes the base command with common setup
func (bc *BaseCommand) Init(ctx context.Context) error {
	// Load configuration
	config, err := bc.LoadConfig(*bc.ConfigFile)
	if err != nil {
		return err
	}
	bc.Config = config

	// Initialize GitHub client using common initialization function
	client, _, err := InitializeGitHubClient(ctx, config)
	if err != nil {
		return err
	}
	bc.GitHubClient = client

	return nil
}
