package main

import (
	"fmt"
	"os"

	"github.com/alan/cherry-picker/cmd/status"
	"github.com/alan/cherry-picker/internal/depmerger"
	"github.com/alan/cherry-picker/internal/refresh"
	"github.com/alan/cherry-picker/internal/state"
	"github.com/spf13/cobra"
)

func newStatusCmd(configFile *string) *cobra.Command {
	var showReleased, showMerged, doFetch bool

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of tracked cherry-pick and dependency PRs",
		Long: `Display the current status of all tracked PRs across both subsystems:
cherry-picks (per target branch) and dependencies. Reads the local state file
without contacting GitHub unless --fetch is given.`,
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			ctx := cobraCmd.Context()

			if doFetch {
				client, st, err := loadStateAndClient(ctx, *configFile)
				if err != nil {
					return fmt.Errorf("failed to load config: %w", err)
				}
				refreshErr := refresh.All(ctx, client, st)
				if err := state.Update(*configFile, func(cur *state.Config) error {
					cur.MergeFetched(st)
					return nil
				}); err != nil {
					return fmt.Errorf("failed to save config: %w", err)
				}
				if refreshErr != nil {
					fmt.Fprintf(os.Stderr, "warning: fetch had errors: %v\n", refreshErr)
				}
			}

			st, err := state.Load(*configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			status.Render(st.CherryView(), *configFile, showReleased)
			fmt.Println()
			configFlag := ""
			if *configFile != defaultConfigFile {
				configFlag = " --config " + *configFile
			}
			depmerger.RenderStatus(os.Stdout, st.DepView(), os.Args[0], configFlag, showMerged)
			return nil
		},
	}

	statusCmd.Flags().BoolVar(&showReleased, "show-released", false, "Show cherry-picks that are completely released")
	statusCmd.Flags().BoolVar(&showMerged, "show-merged", false, "Show dependency PRs that are merged")
	statusCmd.Flags().BoolVar(&doFetch, "fetch", false, "Fetch latest data from GitHub before showing status")

	return statusCmd
}
