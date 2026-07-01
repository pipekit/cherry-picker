package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/config"
	"github.com/alan/cherry-picker/internal/depmerger"
	"github.com/alan/cherry-picker/internal/state"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newMigrateCmd(configFile *string) *cobra.Command {
	var cherryFile, depFile string

	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Build the unified config file from legacy cherry-picks.yaml and dep-merger.yaml",
		Long: `Create the unified config+state file from the two legacy files. Idempotent:
if the unified file already exists it does nothing. Legacy files are left in
place.`,
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runMigrate(*configFile, cherryFile, depFile)
		},
	}

	migrateCmd.Flags().StringVar(&cherryFile, "from-cherry", "cherry-picks.yaml", "Legacy cherry-picker config to import")
	migrateCmd.Flags().StringVar(&depFile, "from-deps", "dep-merger.yaml", "Legacy dep-merger config to import")

	return migrateCmd
}

func runMigrate(unifiedFile, cherryFile, depFile string) error {
	if _, err := os.Stat(unifiedFile); err == nil {
		fmt.Printf("%s already exists; nothing to migrate.\n", unifiedFile)
		return nil
	}

	cherryCfg, _ := config.LoadConfig(cherryFile)
	depCfg, _ := loadLegacyDep(depFile)

	if cherryCfg == nil && depCfg == nil {
		return fmt.Errorf("no legacy config found (looked for %s and %s)", cherryFile, depFile)
	}

	org, repo, err := reconcileRepo(cherryCfg, depCfg)
	if err != nil {
		return err
	}

	unified := &state.Config{Org: org, Repo: repo}

	if cherryCfg != nil {
		unified.LastFetchDate = cherryCfg.LastFetchDate
		unified.CherryPicks = state.CherryPickSection{
			SourceBranch:       cherryCfg.SourceBranch,
			AIAssistantCommand: cherryCfg.AIAssistantCommand,
			LastCheckedRelease: cherryCfg.LastCheckedRelease,
			TrackerIssues:      cherryCfg.TrackerIssues,
			TrackedPRs:         cherryCfg.TrackedPRs,
		}
	}
	if depCfg != nil {
		unified.Dependencies = state.DependencySection{TrackedPRs: depCfg.TrackedPRs}
		unified.LastFetchDate = minTime(unified.LastFetchDate, depCfg.LastFetchDate)
	}

	if err := state.Save(unifiedFile, unified); err != nil {
		return fmt.Errorf("failed to write %s: %w", unifiedFile, err)
	}

	fmt.Printf("Migrated into %s (cherry-picks: %d PRs, dependencies: %d PRs)\n",
		unifiedFile, len(unified.CherryPicks.TrackedPRs), len(unified.Dependencies.TrackedPRs))
	return nil
}

func loadLegacyDep(path string) (*depmerger.Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is from command-line flag
	if err != nil {
		return nil, err
	}
	var c depmerger.Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// reconcileRepo derives org/repo, preferring the cherry config and erroring if
// the two legacy files name different repositories.
func reconcileRepo(cherryCfg *cmd.Config, depCfg *depmerger.Config) (string, string, error) {
	var cOrg, cRepo, dOrg, dRepo string
	if cherryCfg != nil {
		cOrg, cRepo = cherryCfg.Org, cherryCfg.Repo
	}
	if depCfg != nil {
		dOrg, dRepo = depCfg.Org, depCfg.Repo
	}

	if cOrg != "" && dOrg != "" && !strings.EqualFold(cOrg, dOrg) {
		return "", "", fmt.Errorf("org mismatch between legacy files: cherry=%s deps=%s", cOrg, dOrg)
	}
	if cRepo != "" && dRepo != "" && !strings.EqualFold(cRepo, dRepo) {
		return "", "", fmt.Errorf("repo mismatch between legacy files: cherry=%s deps=%s", cRepo, dRepo)
	}

	org := cOrg
	if org == "" {
		org = dOrg
	}
	repo := cRepo
	if repo == "" {
		repo = dRepo
	}
	return org, repo, nil
}

func minTime(a, b *time.Time) *time.Time {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	case b.Before(*a):
		return b
	default:
		return a
	}
}
