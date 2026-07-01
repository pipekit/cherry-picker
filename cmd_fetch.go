package main

import (
	"errors"
	"fmt"

	"github.com/alan/cherry-picker/internal/refresh"
	"github.com/alan/cherry-picker/internal/state"
	"github.com/spf13/cobra"
)

func newFetchCmd(configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "fetch",
		Short: "Fetch cherry-pick and dependency PRs from GitHub",
		Long: `Scrape both subsystems from GitHub and update the tracking file:
merged PRs with cherry-pick/* labels and open PRs with the type/dependencies
label. Partial results are saved even if one subsystem errors.

Requires GITHUB_TOKEN environment variable to be set.`,
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			ctx := cobraCmd.Context()
			client, st, err := loadStateAndClient(ctx, *configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w (run 'config' or 'migrate' first)", err)
			}

			refreshErr := refresh.All(ctx, client, st)

			// Commit whatever was fetched, merging onto the freshly-reloaded
			// on-disk state so a concurrent writer is not clobbered.
			saveErr := state.Update(*configFile, func(cur *state.Config) error {
				cur.MergeFetched(st)
				return nil
			})
			if saveErr != nil {
				return errors.Join(refreshErr, fmt.Errorf("failed to save config: %w", saveErr))
			}
			return refreshErr
		},
	}
}
