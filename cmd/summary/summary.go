// Package summary implements the summary command for generating release notes from tracked commits.
package summary

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/spf13/cobra"
)

// command encapsulates the summary command with common functionality
type command struct {
	commands.BaseCommand
	TargetBranch  string
	PostToTracker bool
}

// NewSummaryCmd creates the summary command
func NewSummaryCmd(globalConfigFile *string, loadConfig func(string) (*cmd.Config, error)) *cobra.Command {
	summaryCmd := &command{}

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
  cherry-picker summary main           # Dev progress for main branch
  cherry-picker summary release-3.7 --post-to-tracker  # Post summary to tracker issue`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			summaryCmd.TargetBranch = args[0]

			// Initialize base command (no save config needed for summary)
			summaryCmd.ConfigFile = globalConfigFile
			summaryCmd.LoadConfig = loadConfig
			if err := summaryCmd.Init(cobraCmd.Context()); err != nil {
				return err
			}

			return summaryCmd.Run(cobraCmd.Context())
		},
	}

	cobraCmd.Flags().BoolVarP(&summaryCmd.PostToTracker, "post-to-tracker", "p", false, "Post summary as comment to tracker issue")

	return cobraCmd
}

// Run executes the summary command
func (sc *command) Run(ctx context.Context) error {
	org := sc.Config.Org
	repo := sc.Config.Repo

	// Create mapping from cherry-pick PR numbers to original PR numbers
	cherryPickMap := createCherryPickMap(sc.Config, sc.TargetBranch)

	slog.Info("Generating summary", "org", org, "repo", repo, "branch", sc.TargetBranch)

	// Fetch latest tags and commits from remote to ensure we have up-to-date data
	if err := fetchGitData(ctx, sc.TargetBranch); err != nil {
		slog.Warn("Failed to fetch git data from remote, using local data", "error", err)
	}

	// Get the last release tag for this branch from local git
	lastTag, err := getLastReleaseTag(ctx, sc.TargetBranch)
	if err != nil {
		return fmt.Errorf("failed to get last release tag: %w", err)
	}

	// Generate next version
	nextVersion, err := incrementPatchVersion(lastTag)
	if err != nil {
		return fmt.Errorf("failed to increment version: %w", err)
	}

	// Get commits since the last tag from local git
	commits, err := getCommitsSinceTag(ctx, sc.TargetBranch, lastTag)
	if err != nil {
		return fmt.Errorf("failed to get commits: %w", err)
	}

	// Get picked PRs that might not be in commits yet
	pickedPRs := getPickedPRs(sc.Config, sc.TargetBranch)

	// Generate markdown summary
	summary := generateMarkdownSummary(nextVersion, lastTag, sc.TargetBranch, commits, cherryPickMap, pickedPRs)

	// Print summary to stdout
	fmt.Print(summary)

	// Handle posting to tracker if requested
	if sc.PostToTracker {
		return sc.postToTrackerIssue(ctx, nextVersion, summary)
	}

	return nil
}
