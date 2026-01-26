package main

import (
	"fmt"
	"log/slog"

	"github.com/alan/cherry-picker/internal/git"
	"github.com/spf13/cobra"
)

func newConfigCmd(globalConfigFile *string) *cobra.Command {
	var (
		org  string
		repo string
	)

	cobraCmd := &cobra.Command{
		Use:   "config",
		Short: "Initialize a new dep-merger.yaml configuration file",
		Long: `Config creates a new dep-merger.yaml file with the specified
organization and repository configuration.

When run from a git repository root, it will automatically detect the organization
and repository from the git remote origin.`,
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runConfigWithGitDetection(*globalConfigFile, org, repo)
		},
	}

	cobraCmd.Flags().StringVarP(&org, "org", "o", "", "GitHub organization or username (auto-detected from git if available)")
	cobraCmd.Flags().StringVarP(&repo, "repo", "r", "", "GitHub repository name (auto-detected from git if available)")

	return cobraCmd
}

func runConfigWithGitDetection(configFile, org, repo string) error {
	// Load existing config first to see what we already have
	config, _ := loadOrCreateConfig(configFile)

	// Start with provided values, fall back to existing config values
	finalOrg := org
	if finalOrg == "" {
		finalOrg = config.Org
	}

	finalRepo := repo
	if finalRepo == "" {
		finalRepo = config.Repo
	}

	// Try git detection for any still-missing values
	if finalOrg == "" || finalRepo == "" {
		if gitInfo, err := detectGitRepoInfo(); err == nil {
			if finalOrg == "" {
				finalOrg = gitInfo.Org
				slog.Info("Auto-detected organization", "org", finalOrg)
			}
			if finalRepo == "" {
				finalRepo = gitInfo.Repo
				slog.Info("Auto-detected repository", "repo", finalRepo)
			}
		}
	}

	// Validate required fields
	if finalOrg == "" {
		return fmt.Errorf("organization is required (use --org flag or run from a git repository)")
	}
	if finalRepo == "" {
		return fmt.Errorf("repository is required (use --repo flag or run from a git repository)")
	}

	return runConfig(configFile, finalOrg, finalRepo)
}

func runConfig(configFile, org, repo string) error {
	config, isUpdate := loadOrCreateConfig(configFile)

	// Update config with provided values
	if org != "" {
		config.Org = org
	}
	if repo != "" {
		config.Repo = repo
	}

	if err := SaveConfig(configFile, config); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	action := "initialized"
	if isUpdate {
		action = "updated"
	}
	fmt.Printf("Successfully %s %s with:\n", action, configFile)
	fmt.Printf("  Organization: %s\n", config.Org)
	fmt.Printf("  Repository: %s\n", config.Repo)
	return nil
}

func loadOrCreateConfig(configFile string) (*Config, bool) {
	if config, err := LoadConfig(configFile); err == nil {
		return config, true
	}
	return &Config{}, false
}

// detectGitRepoInfo attempts to detect git repository information
func detectGitRepoInfo() (*git.RepoInfo, error) {
	return git.DetectRepoInfo()
}
