package fetch

import (
	"fmt"
	"time"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/spf13/cobra"
)

// FetchCommand encapsulates the fetch command with common functionality
type FetchCommand struct {
	commands.BaseCommand
	SinceDate string
}

// NewFetchCmd creates and returns the fetch command
func NewFetchCmd(globalConfigFile *string, loadConfig func(string) (*cmd.Config, error), saveConfig func(string, *cmd.Config) error) *cobra.Command {
	fetchCmd := &FetchCommand{}

	command := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch new merged PRs from GitHub that need cherry-picking decisions",
		Long: `Fetch new merged PRs from the source branch since the last fetch date
(or a specified date) and interactively ask whether to pick or ignore each one.

Requires GITHUB_TOKEN environment variable to be set.`,
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			// Initialize base command
			fetchCmd.ConfigFile = globalConfigFile
			fetchCmd.LoadConfig = loadConfig
			fetchCmd.SaveConfig = saveConfig
			if err := fetchCmd.Init(); err != nil {
				return err
			}

			return fetchCmd.Run()
		},
	}

	command.Flags().StringVarP(&fetchCmd.SinceDate, "since", "s", "", "Fetch PRs since this date (YYYY-MM-DD), defaults to last fetch date")

	return command
}

// Run executes the fetch command
func (fc *FetchCommand) Run() error {
	since, err := determineSinceDate(fc.SinceDate, fc.Config.LastFetchDate)
	if err != nil {
		return err
	}

	return fetchAndProcessPRs(*fc.ConfigFile, fc.Config, since, fc.SaveConfig)
}

// determineSinceDate determines the date to fetch PRs from
func determineSinceDate(sinceDate string, lastFetchDate *time.Time) (time.Time, error) {
	if sinceDate != "" {
		since, err := time.Parse("2006-01-02", sinceDate)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid date format, use YYYY-MM-DD: %w", err)
		}
		return since, nil
	}

	if lastFetchDate != nil {
		return *lastFetchDate, nil
	}

	return time.Now().AddDate(0, 0, -30), nil
}
