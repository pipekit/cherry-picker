package config

import (
	"fmt"

	"os/exec"
	"regexp"
	"strings"

	"github.com/alan/cherry-picker/cmd"
	"github.com/spf13/cobra"
)

// NewConfigCmd creates and returns the config command
func NewConfigCmd(globalConfigFile *string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) *cobra.Command {
	var (
		org            string
		repo           string
		sourceBranch   string
		targetBranches []string
	)

	cmd := createConfigCommand(globalConfigFile, &org, &repo, &sourceBranch, &targetBranches, loadConfig, saveConfig)
	addConfigFlags(cmd, &org, &repo, &sourceBranch, &targetBranches)
	// Note: org and repo are no longer marked as required since they can be auto-detected from git

	return cmd
}

// createConfigCommand creates the basic config command structure
func createConfigCommand(globalConfigFile *string, org, repo, sourceBranch *string, targetBranches *[]string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Initialize a new cherry-picks.yaml configuration file",
		Long: `Config creates a new cherry-picks.yaml file with the specified
organization, repository, source branch, and target branches configuration.

When run from a git repository root, it will automatically detect the organization,
repository, and current branch from the git remote origin.

The source branch defaults to 'main' if not specified and not detected from git.
Target branches can be specified as a comma-separated list.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigWithGitDetection(*globalConfigFile, *org, *repo, *sourceBranch, *targetBranches, loadConfig, saveConfig)
		},
	}
}

// addConfigFlags adds all flags to the config command
func addConfigFlags(cmd *cobra.Command, org, repo, sourceBranch *string, targetBranches *[]string) {
	cmd.Flags().StringVarP(org, "org", "o", "", "GitHub organization or username (auto-detected from git if available)")
	cmd.Flags().StringVarP(repo, "repo", "r", "", "GitHub repository name (auto-detected from git if available)")
	cmd.Flags().StringVarP(sourceBranch, "source-branch", "s", "", "Source branch name (auto-detected from git if available, defaults to 'main')")
	cmd.Flags().StringSliceVarP(targetBranches, "target-branches", "t", []string{}, "Target branches for cherry-picking (comma-separated)")
}

// runConfigWithGitDetection handles config creation with git auto-detection
func runConfigWithGitDetection(configFile, org, repo, sourceBranch string, targetBranches []string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) error {
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

	// Try git detection for any still-missing values
	if finalOrg == "" || finalRepo == "" || finalSourceBranch == "" {
		if gitInfo, err := detectGitRepoInfo(); err == nil {
			if finalOrg == "" {
				finalOrg = gitInfo.Org
				fmt.Printf("🔍 Auto-detected organization: %s\n", finalOrg)
			}
			if finalRepo == "" {
				finalRepo = gitInfo.Repo
				fmt.Printf("🔍 Auto-detected repository: %s\n", finalRepo)
			}
			if finalSourceBranch == "" {
				finalSourceBranch = gitInfo.SourceBranch
				fmt.Printf("🔍 Auto-detected source branch: %s\n", finalSourceBranch)
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

	return runConfig(configFile, finalOrg, finalRepo, finalSourceBranch, targetBranches, loadConfig, saveConfig)
}

func runConfig(configFile, org, repo, sourceBranch string, targetBranches []string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) error {
	config, isUpdate := loadOrCreateConfig(configFile, loadConfig)

	// Update config with provided values
	updateConfigWithProvidedValues(config, org, repo, sourceBranch, targetBranches)

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
	if len(config.TargetBranches) > 0 {
		fmt.Printf("  Target Branches: %v\n", config.TargetBranches)
	}
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
func updateConfigWithProvidedValues(config *cmd.Config, org, repo, sourceBranch string, targetBranches []string) {
	if org != "" {
		config.Org = org
	}
	if repo != "" {
		config.Repo = repo
	}
	if sourceBranch != "" {
		config.SourceBranch = sourceBranch
	}
	if len(targetBranches) > 0 {
		config.TargetBranches = targetBranches
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
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// parseGitRemote extracts org and repo from git remote origin
func parseGitRemote() (string, string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
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
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "", fmt.Errorf("unable to determine current branch")
	}

	return branch, nil
}
