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

	// Initialize GitHub client using common initialization function
	client, ctx, err := InitializeGitHubClient(config)
	if err != nil {
		return err
	}
	bc.GitHubClient = client
	bc.Context = ctx

	return nil
}
