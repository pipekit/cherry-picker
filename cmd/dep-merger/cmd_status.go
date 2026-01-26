package main

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
)

func newStatusCmd(globalConfigFile *string) *cobra.Command {
	var showMerged bool
	var doFetch bool

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of tracked dependency PRs",
		Long: `Display the current status of all tracked dependency PRs.
Shows which PRs are pending, have passing/failing CI, or are merged.
By default, hides PRs that have been merged.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runStatus(cmd.Context(), *globalConfigFile, showMerged, doFetch)
		},
	}

	statusCmd.Flags().BoolVar(&showMerged, "show-merged", false, "Show PRs that have been merged")
	statusCmd.Flags().BoolVar(&doFetch, "fetch", false, "Fetch latest data from GitHub before showing status")

	return statusCmd
}

func runStatus(ctx context.Context, configFile string, showMerged bool, doFetch bool) error {
	config, err := LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w (run 'dep-merger config' first)", err)
	}

	// Run fetch if requested
	if doFetch {
		if err := runFetch(ctx, configFile, config); err != nil {
			return fmt.Errorf("fetch failed: %w", err)
		}
		// Reload config after fetch
		config, err = LoadConfig(configFile)
		if err != nil {
			return fmt.Errorf("failed to reload config after fetch: %w", err)
		}
	}

	if len(config.TrackedPRs) == 0 {
		fmt.Println("No dependency PRs tracked.")
		fmt.Println("Run 'dep-merger fetch' to discover dependency PRs.")
		return nil
	}

	// Filter PRs
	prsToDisplay := config.TrackedPRs
	if !showMerged {
		prsToDisplay = filterNonMerged(config.TrackedPRs)
	}

	if len(prsToDisplay) == 0 {
		fmt.Println("No active dependency PRs. All PRs are merged.")
		fmt.Println("Use --show-merged to see merged PRs.")
		return nil
	}

	// Sort by PR number
	sort.Slice(prsToDisplay, func(i, j int) bool {
		return prsToDisplay[i].Number < prsToDisplay[j].Number
	})

	// Display header
	fmt.Printf("Dependency PR status for %s/%s\n\n", config.Org, config.Repo)

	// Display each PR
	for _, pr := range prsToDisplay {
		displayPRStatus(pr, config, configFile)
	}

	// Display summary
	displaySummary(prsToDisplay)

	return nil
}

func filterNonMerged(prs []TrackedPR) []TrackedPR {
	var filtered []TrackedPR
	for _, pr := range prs {
		if !pr.Merged {
			filtered = append(filtered, pr)
		}
	}
	return filtered
}

func displayPRStatus(pr TrackedPR, config *Config, configFile string) {
	url := fmt.Sprintf("https://github.com/%s/%s/pull/%d", config.Org, config.Repo, pr.Number)

	if pr.Title != "" {
		fmt.Printf("%s (%s)\n", pr.Title, url)
	} else {
		fmt.Printf("%s\n", url)
	}

	executablePath := os.Args[0]
	configFlag := getConfigFlag(configFile)

	if pr.Merged {
		fmt.Printf("  Status: ✅ merged\n")
	} else {
		switch pr.CIStatus {
		case CIStatusPassing:
			fmt.Printf("  Status: ✅ CI passing\n")
			fmt.Printf("  💡 %s%s merge %d\n", executablePath, configFlag, pr.Number)
		case CIStatusFailing:
			fmt.Printf("  Status: ❌ CI failing")
			if pr.RunAttempt > 0 {
				fmt.Printf(" [run attempt %d]", pr.RunAttempt)
			}
			fmt.Println()
			fmt.Printf("  💡 %s%s retry %d\n", executablePath, configFlag, pr.Number)
		case CIStatusPending:
			fmt.Printf("  Status: 🔄 CI pending\n")
		default:
			fmt.Printf("  Status: ❓ CI unknown\n")
		}
	}
	fmt.Println()
}

func displaySummary(prs []TrackedPR) {
	passing := 0
	failing := 0
	pending := 0
	merged := 0

	for _, pr := range prs {
		if pr.Merged {
			merged++
			continue
		}
		switch pr.CIStatus {
		case CIStatusPassing:
			passing++
		case CIStatusFailing:
			failing++
		case CIStatusPending:
			pending++
		}
	}

	fmt.Printf("Summary: %d PR(s) - %d passing, %d failing, %d pending, %d merged\n",
		len(prs), passing, failing, pending, merged)
}

func getConfigFlag(configFile string) string {
	if configFile == "dep-merger.yaml" {
		return ""
	}
	return fmt.Sprintf(" --config %s", configFile)
}
