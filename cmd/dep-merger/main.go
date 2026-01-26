// package main is the entry point for the dep-merger tool
package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	var configFile string
	var logLevel string
	var logFormat string

	rootCmd := &cobra.Command{
		Use:   "dep-merger",
		Short: "A CLI tool for managing dependency PRs on GitHub repositories",
		Long: `dep-merger is a CLI tool that helps manage dependency PRs
(those with type/dependencies label) by tracking their CI status
and enabling retry and merge operations.`,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			setupLogger(logLevel, logFormat)
		},
	}

	// Add global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "dep-merger.yaml", "Configuration file path")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVarP(&logFormat, "log-format", "f", "text", "Log format (text, json)")

	// Add commands
	rootCmd.AddCommand(newConfigCmd(&configFile))
	rootCmd.AddCommand(newFetchCmd(&configFile))
	rootCmd.AddCommand(newStatusCmd(&configFile))
	rootCmd.AddCommand(newRetryCmd(&configFile))
	rootCmd.AddCommand(newMergeCmd(&configFile))
	rootCmd.AddCommand(newApproveCmd(&configFile))

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
