package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"github.com/alan/cherry-picker/internal/commands"
	"github.com/alan/cherry-picker/internal/github"
	"github.com/alan/cherry-picker/internal/refresh"
	"github.com/alan/cherry-picker/internal/state"
	"github.com/spf13/cobra"
)

func newDaemonCmd(configFile *string) *cobra.Command {
	var interval time.Duration

	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run a background poller that keeps the state file fresh",
		Long: `Run an initial full scrape of both subsystems, then re-scrape on an
interval. After each tick the unified state file is updated atomically, so CLI
commands (status, merge, ...) always read fresh data without waiting on GitHub.
Runs in the foreground; stop with Ctrl-C.

Requires GITHUB_TOKEN environment variable to be set.`,
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return runDaemon(cobraCmd.Context(), *configFile, interval)
		},
	}

	daemonCmd.Flags().DurationVar(&interval, "interval", 5*time.Minute, "Polling interval between full scrapes")

	return daemonCmd
}

func runDaemon(parent context.Context, configFile string, interval time.Duration) error {
	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Build the GitHub client once from the persisted org/repo.
	st, err := state.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w (run 'config' or 'migrate' first)", err)
	}
	client, _, err := commands.InitializeGitHubClient(ctx, st.CherryView())
	if err != nil {
		return err
	}

	slog.Info("daemon starting", "config", configFile, "interval", interval.String())

	// Immediate full scrape, then on the interval.
	daemonTick(ctx, client, configFile)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("daemon shutting down")
			return nil
		case <-ticker.C:
			daemonTick(ctx, client, configFile)
		}
	}
}

// daemonTick runs one full scrape and commits it. Errors are logged and
// swallowed so the daemon keeps running; the next tick self-heals from GitHub.
func daemonTick(ctx context.Context, client *github.Client, configFile string) {
	start := time.Now()
	slog.Info("tick starting")

	snap, err := state.Load(configFile)
	if err != nil {
		slog.Error("tick: failed to load state", "error", err)
		return
	}

	refreshErr := refresh.All(ctx, client, snap)

	if err := state.Update(configFile, func(cur *state.Config) error {
		cur.MergeFetched(snap)
		return nil
	}); err != nil {
		slog.Error("tick: failed to save state", "error", err)
	}
	if refreshErr != nil {
		slog.Warn("tick: refresh had errors", "error", refreshErr)
	}

	slog.Info("tick complete", "duration", time.Since(start).String())
}
