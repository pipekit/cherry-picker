// Package config implements the config command for initializing and updating cherry-picker configuration.
package config

import (
	"fmt"
	"log/slog"

	"os/exec"
	"regexp"
	"strings"

	"github.com/alan/cherry-picker/cmd"
	"github.com/spf13/cobra"
)

// NewConfigCmd creates and returns the config command
func NewConfigCmd(globalConfigFile *string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) *cobra.Command {
	var (
		org                string
		repo               string
		sourceBranch       string
		aiAssistantCommand string
	)

	cobraCmd := createConfigCommand(globalConfigFile, &org, &repo, &sourceBranch, &aiAssistantCommand, loadConfig, saveConfig)
	addConfigFlags(cobraCmd, &org, &repo, &sourceBranch, &aiAssistantCommand)
	// Note: org and repo are no longer marked as required since they can be auto-detected from git

	return cobraCmd
}

// createConfigCommand creates the basic config command structure
func createConfigCommand(globalConfigFile *string, org, repo, sourceBranch, aiAssistantCommand *string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Initialize a new cherry-picks.yaml configuration file",
		Long: `Config creates a new cherry-picks.yaml file with the specified
organization, repository, and source branch configuration.

When run from a git repository root, it will automatically detect the organization,
repository, and current branch from the git remote origin.

The source branch defaults to 'main' if not specified and not detected from git.
Target branches are determined automatically from cherry-pick/* labels on PRs.
AI assistant command is required for conflict resolution (e.g., 'cursor-agent' or 'claude').`,
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runConfigWithGitDetection(*globalConfigFile, *org, *repo, *sourceBranch, *aiAssistantCommand, loadConfig, saveConfig)
		},
	}
}

// addConfigFlags adds all flags to the config command
func addConfigFlags(cobraCmd *cobra.Command, org, repo, sourceBranch, aiAssistantCommand *string) {
	cobraCmd.Flags().StringVarP(org, "org", "o", "", "GitHub organization or username (auto-detected from git if available)")
	cobraCmd.Flags().StringVarP(repo, "repo", "r", "", "GitHub repository name (auto-detected from git if available)")
	cobraCmd.Flags().StringVarP(sourceBranch, "source-branch", "s", "", "Source branch name (auto-detected from git if available, defaults to 'main')")
	cobraCmd.Flags().StringVarP(aiAssistantCommand, "ai-assistant", "a", "", "AI assistant command for conflict resolution (e.g., 'cursor-agent', 'claude')")
}

// runConfigWithGitDetection handles config creation with git auto-detection
func runConfigWithGitDetection(configFile, org, repo, sourceBranch, aiAssistantCommand string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) error {
	// Load existing config first to see what we already have
	config, _ := loadOrCreateConfig(configFile, loadConfig)

	// Start with provided values, fall back to existing config values
	finalOrg := org
	if finalOrg == "" {
		finalOrg = config.Org
	}

	finalRepo := repo
	if finalRepo == "" {
		finalRepo = config.Repo
	}

	finalSourceBranch := sourceBranch
	if finalSourceBranch == "" {
		finalSourceBranch = config.SourceBranch
	}

	finalAIAssistant := aiAssistantCommand
	if finalAIAssistant == "" {
		finalAIAssistant = config.AIAssistantCommand
	}

	// Try git detection for any still-missing values
	if finalOrg == "" || finalRepo == "" || finalSourceBranch == "" {
		if gitInfo, err := detectGitRepoInfo(); err == nil {
			if finalOrg == "" {
				finalOrg = gitInfo.Org
				slog.Info("Auto-detected organization", "org", finalOrg)
			}
			if finalRepo == "" {
				finalRepo = gitInfo.Repo
				slog.Info("Auto-detected repository", "repo", finalRepo)
			}
			if finalSourceBranch == "" {
				finalSourceBranch = gitInfo.SourceBranch
				slog.Info("Auto-detected source branch", "branch", finalSourceBranch)
			}
		} else {
			// Fall back to defaults for source branch
			if finalSourceBranch == "" {
				finalSourceBranch = "main"
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
	if finalAIAssistant == "" {
		return fmt.Errorf("AI assistant command is required (use --ai-assistant flag, e.g., 'cursor-agent' or 'claude')")
	}

	return runConfig(configFile, finalOrg, finalRepo, finalSourceBranch, finalAIAssistant, loadConfig, saveConfig)
}

func runConfig(configFile, org, repo, sourceBranch, aiAssistantCommand string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) error {
	config, isUpdate := loadOrCreateConfig(configFile, loadConfig)

	// Update config with provided values
	updateConfigWithProvidedValues(config, org, repo, sourceBranch, aiAssistantCommand)

	if err := saveConfig(configFile, config); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	displayConfigSuccess(configFile, config, isUpdate)
	return nil
}

// displayConfigSuccess shows the configuration success message
func displayConfigSuccess(configFile string, config *cmd.Config, isUpdate bool) {
	action := "initialized"
	if isUpdate {
		action = "updated"
	}
	fmt.Printf("Successfully %s %s with:\n", action, configFile)
	fmt.Printf("  Organization: %s\n", config.Org)
	fmt.Printf("  Repository: %s\n", config.Repo)
	fmt.Printf("  Source Branch: %s\n", config.SourceBranch)
	fmt.Printf("  AI Assistant: %s\n", config.AIAssistantCommand)
}

// loadOrCreateConfig loads existing config or creates a new one
func loadOrCreateConfig(configFile string, loadConfig func(string) (*cmd.Config, error)) (*cmd.Config, bool) {
	if config, err := loadConfig(configFile); err == nil {
		// File exists and was loaded successfully
		return config, true
	}

	// File doesn't exist or couldn't be loaded, create new config
	return &cmd.Config{}, false
}

// updateConfigWithProvidedValues updates config with any non-empty provided values
func updateConfigWithProvidedValues(config *cmd.Config, org, repo, sourceBranch, aiAssistantCommand string) {
	if org != "" {
		config.Org = org
	}
	if repo != "" {
		config.Repo = repo
	}
	if sourceBranch != "" {
		config.SourceBranch = sourceBranch
	}
	if aiAssistantCommand != "" {
		config.AIAssistantCommand = aiAssistantCommand
	}
}

// GitRepoInfo holds detected git repository information
type GitRepoInfo struct {
	Org          string
	Repo         string
	SourceBranch string
}

// detectGitRepoInfo attempts to detect git repository information
func detectGitRepoInfo() (*GitRepoInfo, error) {
	if !isGitRepository() {
		return nil, fmt.Errorf("not in a git repository")
	}

	org, repo, err := parseGitRemote()
	if err != nil {
		return nil, fmt.Errorf("failed to parse git remote: %w", err)
	}

	sourceBranch, err := getCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	return &GitRepoInfo{
		Org:          org,
		Repo:         repo,
		SourceBranch: sourceBranch,
	}, nil
}

// isGitRepository checks if current directory is in a git repository
func isGitRepository() bool {
	gitCmd := exec.Command("git", "rev-parse", "--git-dir")
	return gitCmd.Run() == nil
}

// parseGitRemote extracts org and repo from git remote origin
func parseGitRemote() (string, string, error) {
	gitCmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := gitCmd.Output()
	if err != nil {
		return "", "", err
	}

	remoteURL := strings.TrimSpace(string(output))
	return parseRemoteURL(remoteURL)
}

// parseRemoteURL extracts org and repo from various GitHub URL formats
func parseRemoteURL(remoteURL string) (string, string, error) {
	// Handle SSH format: git@github.com:org/repo.git
	sshRegex := regexp.MustCompile(`git@github\.com:([^/]+)/([^/]+?)(?:\.git)?$`)
	if matches := sshRegex.FindStringSubmatch(remoteURL); len(matches) == 3 {
		return matches[1], matches[2], nil
	}

	// Handle HTTPS format: https://github.com/org/repo.git
	httpsRegex := regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+?)(?:\.git)?/?$`)
	if matches := httpsRegex.FindStringSubmatch(remoteURL); len(matches) == 3 {
		return matches[1], matches[2], nil
	}

	return "", "", fmt.Errorf("unable to parse GitHub remote URL: %s", remoteURL)
}

// getCurrentBranch gets the current git branch name
func getCurrentBranch() (string, error) {
	gitCmd := exec.Command("git", "branch", "--show-current")
	output, err := gitCmd.Output()
	if err != nil {
		return "", err
	}

	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "", fmt.Errorf("unable to determine current branch")
	}

	return branch, nil
}
