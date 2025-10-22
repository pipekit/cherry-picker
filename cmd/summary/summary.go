package summary

import (
	"fmt"
	"log/slog"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/spf13/cobra"
)

// SummaryCommand encapsulates the summary command with common functionality
type SummaryCommand struct {
	commands.BaseCommand
	TargetBranch string
}

// NewSummaryCmd creates the summary command
func NewSummaryCmd(globalConfigFile *string, loadConfig func(string) (*cmd.Config, error)) *cobra.Command {
	summaryCmd := &SummaryCommand{}

	cobraCmd := &cobra.Command{
		Use:   "summary <target-branch>",
		Short: "Generate development progress summary for a branch",
		Long: `Generate a markdown summary of development progress on the target branch since the last release.

This command shows both merged commits and work-in-progress cherry-picks from your config file.
It queries GitHub directly and uses the org/repo from the config file to show:
- [x] Completed work (merged commits and cherry-picks)
- [ ] In-progress work (picked but not yet merged)

Examples:
  cherry-picker summary release-3.7    # Dev progress for release-3.7 branch
  cherry-picker summary main           # Dev progress for main branch`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			summaryCmd.TargetBranch = args[0]

			// Initialize base command (no save config needed for summary)
			summaryCmd.ConfigFile = globalConfigFile
			summaryCmd.LoadConfig = loadConfig
			if err := summaryCmd.Init(); err != nil {
				return err
			}

			return summaryCmd.Run()
		},
	}

	return cobraCmd
}

// Run executes the summary command
func (sc *SummaryCommand) Run() error {
	org := sc.Config.Org
	repo := sc.Config.Repo

	// Create mapping from cherry-pick PR numbers to original PR numbers
	cherryPickMap := createCherryPickMap(sc.Config, sc.TargetBranch)

	slog.Info("Generating summary", "org", org, "repo", repo, "branch", sc.TargetBranch)

	// Get the last release tag for this branch
	lastTag, err := getLastReleaseTag(sc.GitHubClient, sc.TargetBranch)
	if err != nil {
		return fmt.Errorf("failed to get last release tag: %w", err)
	}

	// Generate next version
	nextVersion, err := incrementPatchVersion(lastTag)
	if err != nil {
		return fmt.Errorf("failed to increment version: %w", err)
	}

	// Get commits since the last tag
	commits, err := getCommitsSinceTag(sc.GitHubClient, sc.TargetBranch, lastTag)
	if err != nil {
		return fmt.Errorf("failed to get commits: %w", err)
	}

	// Get picked PRs that might not be in commits yet
	pickedPRs := getPickedPRs(sc.Config, sc.TargetBranch)

	// Get open PRs targeting this branch
	openPRs, err := sc.GitHubClient.GetOpenPRs(sc.TargetBranch)
	if err != nil {
		return fmt.Errorf("failed to get open PRs: %w", err)
	}

	// Generate markdown output
	generateMarkdownSummary(nextVersion, lastTag, sc.TargetBranch, commits, cherryPickMap, pickedPRs, openPRs)

	return nil
}
