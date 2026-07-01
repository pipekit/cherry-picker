// package main is the entry point for the cherry-pick tool
package main

import (
	"log/slog"
	"os"

	configcmd "github.com/alan/cherry-picker/cmd/config"
	"github.com/alan/cherry-picker/cmd/pick"
	"github.com/alan/cherry-picker/cmd/summary"
	"github.com/spf13/cobra"
)

func main() {
	var configFile string
	var logLevel string
	var logFormat string

	rootCmd := &cobra.Command{
		Use:   "cherry-picker",
		Short: "Manage cherry-picks and dependency PRs across GitHub repositories",
		Long: `cherry-picker manages cherry-picks across release branches and dependency
PRs for a GitHub repository, tracking their state in a single YAML file. Run the
daemon to keep that state fresh in the background so interactive commands are
instant.`,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			setupLogger(logLevel, logFormat)
		},
	}

	// Add global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", defaultConfigFile, "Configuration file path")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVarP(&logFormat, "log-format", "f", "text", "Log format (text, json)")

	// Cherry-pick-only commands, wired to the unified state via adapters.
	rootCmd.AddCommand(configcmd.NewConfigCmd(&configFile, loadCherry, saveCherry))
	rootCmd.AddCommand(pick.NewPickCmd(&configFile, loadCherry, saveCherry))
	rootCmd.AddCommand(summary.NewSummaryCmd(&configFile, loadCherry))

	// Unified commands spanning both subsystems.
	rootCmd.AddCommand(newFetchCmd(&configFile))
	rootCmd.AddCommand(newStatusCmd(&configFile))
	rootCmd.AddCommand(newMergeCmd(&configFile))
	rootCmd.AddCommand(newRetryCmd(&configFile))
	rootCmd.AddCommand(newApproveCmd(&configFile))
	rootCmd.AddCommand(newMigrateCmd(&configFile))
	rootCmd.AddCommand(newDaemonCmd(&configFile))

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func setupLogger(level, format string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}

	slog.SetDefault(slog.New(handler))
}
