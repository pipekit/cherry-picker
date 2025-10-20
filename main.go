package main

import (
	"os"

	configcmd "github.com/alan/cherry-picker/cmd/config"
	fetchcmd "github.com/alan/cherry-picker/cmd/fetch"
	"github.com/alan/cherry-picker/cmd/merge"
	"github.com/alan/cherry-picker/cmd/pick"
	"github.com/alan/cherry-picker/cmd/retry"
	"github.com/alan/cherry-picker/cmd/status"
	"github.com/alan/cherry-picker/cmd/summary"
	"github.com/alan/cherry-picker/internal/config"
	"github.com/spf13/cobra"
)

func main() {
	var configFile string

	rootCmd := &cobra.Command{
		Use:   "cherry-picker",
		Short: "A CLI tool for managing cherry-picks across GitHub repositories",
		Long: `cherry-picker is a CLI tool that helps manage cherry-picking commits
across GitHub repositories using a YAML configuration file to track state.`,
	}

	// Add global config flag
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "cherry-picks.yaml", "Configuration file path")

	// Create commands with access to the global config file
	rootCmd.AddCommand(configcmd.NewConfigCmd(&configFile, config.LoadConfig, config.SaveConfig))
	rootCmd.AddCommand(fetchcmd.NewFetchCmd(&configFile, config.LoadConfig, config.SaveConfig))
	rootCmd.AddCommand(status.NewStatusCmd(&configFile, config.LoadConfig))
	rootCmd.AddCommand(pick.NewPickCmd(&configFile, config.LoadConfig, config.SaveConfig))
	rootCmd.AddCommand(retry.NewRetryCmd(config.LoadConfig, config.SaveConfig))
	rootCmd.AddCommand(merge.NewMergeCmd(config.LoadConfig, config.SaveConfig))
	rootCmd.AddCommand(summary.NewSummaryCmd(&configFile, config.LoadConfig))

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
