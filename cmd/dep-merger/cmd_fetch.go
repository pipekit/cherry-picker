package main

import (
	"context"
	"fmt"
	"time"

	"github.com/alan/cherry-picker/internal/github"
	"github.com/spf13/cobra"
)

const dependenciesLabel = "type/dependencies"

func newFetchCmd(globalConfigFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "fetch",
		Short: "Fetch open dependency PRs from GitHub",
		Long: `Fetch open PRs with the type/dependencies label from GitHub
and add them to the tracking file.

Requires GITHUB_TOKEN environment variable to be set.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			config, err := LoadConfig(*globalConfigFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w (run 'dep-merger config' first)", err)
			}

			return runFetch(cmd.Context(), *globalConfigFile, config)
		},
	}
}

func runFetch(ctx context.Context, configFile string, config *Config) error {
	client, err := initGitHubClient(ctx, config)
	if err != nil {
		return err
	}

	fmt.Printf("Fetching open PRs with label '%s' from %s/%s...\n", dependenciesLabel, config.Org, config.Repo)

	prs, err := client.GetOpenPRsWithLabel(ctx, dependenciesLabel)
	if err != nil {
		return fmt.Errorf("failed to fetch PRs: %w", err)
	}

	if len(prs) == 0 {
		fmt.Println("No open dependency PRs found.")
		now := time.Now()
		config.LastFetchDate = &now
		return SaveConfig(configFile, config)
	}

	fmt.Printf("Found %d open dependency PR(s)\n", len(prs))

	// Update tracked PRs
	newCount := 0
	updatedCount := 0

	for _, pr := range prs {
		existing := findTrackedPR(config, pr.Number)
		if existing != nil {
			// Update existing PR with latest details
			prDetails, err := client.GetPRWithDetailsNoDCOFilter(ctx, pr.Number)
			if err != nil {
				fmt.Printf("  Warning: failed to get details for PR #%d: %v\n", pr.Number, err)
				continue
			}
			existing.Title = prDetails.Title
			existing.CIStatus = ParseCIStatus(prDetails.CIStatus)
			existing.RunAttempt = prDetails.RunAttempt
			existing.FailingChecks = prDetails.FailingChecks
			updatedCount++
		} else {
			// Fetch full details for new PR
			prDetails, err := client.GetPRWithDetailsNoDCOFilter(ctx, pr.Number)
			if err != nil {
				fmt.Printf("  Warning: failed to get details for PR #%d: %v\n", pr.Number, err)
				continue
			}
			config.TrackedPRs = append(config.TrackedPRs, TrackedPR{
				Number:        prDetails.Number,
				Title:         prDetails.Title,
				CIStatus:      ParseCIStatus(prDetails.CIStatus),
				RunAttempt:    prDetails.RunAttempt,
				FailingChecks: prDetails.FailingChecks,
				Merged:        false,
			})
			newCount++
			fmt.Printf("  Added: #%d %s\n", prDetails.Number, prDetails.Title)
		}
	}

	// Mark PRs that are no longer open (merged or closed)
	markClosedPRs(config, prs)

	// Update last fetch date
	now := time.Now()
	config.LastFetchDate = &now

	if err := SaveConfig(configFile, config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Fetch complete: %d new, %d updated\n", newCount, updatedCount)
	return nil
}

func markClosedPRs(config *Config, openPRs []github.PR) {
	openPRNumbers := make(map[int]bool)
	for _, pr := range openPRs {
		openPRNumbers[pr.Number] = true
	}

	for i := range config.TrackedPRs {
		if !config.TrackedPRs[i].Merged && !openPRNumbers[config.TrackedPRs[i].Number] {
			// PR is no longer open and wasn't merged by us - assume it was merged externally
			config.TrackedPRs[i].Merged = true
		}
	}
}
