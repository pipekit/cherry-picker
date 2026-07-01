package main

import (
	"fmt"

	"github.com/alan/cherry-picker/internal/commands"
	"github.com/alan/cherry-picker/internal/depmerger"
	"github.com/spf13/cobra"
)

func newApproveCmd(configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "approve [pr-number]",
		Short: "Approve dependency PRs with passing CI",
		Long: `Approve dependency PRs (those with the type/dependencies label). With no
PR number, approves all not-yet-approved dependency PRs with passing CI; with a
PR number, approves that dependency PR. Useful for PRs with auto-merge enabled.

Requires GITHUB_TOKEN environment variable to be set.`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			ctx := cobraCmd.Context()
			prNumber, err := commands.ParsePRNumberFromArgs(args, false)
			if err != nil {
				return err
			}

			client, st, err := loadStateAndClient(ctx, *configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			dv := st.DepView()
			if err := depmerger.ApprovePRs(ctx, client, dv, prNumber); err != nil {
				return err
			}
			return saveDep(*configFile, dv)
		},
	}
}
