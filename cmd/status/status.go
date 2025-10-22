// Package status implements the status command for displaying tracked PRs and their cherry-pick status.
package status

import (
	"fmt"
	"os"
	"sort"

	"github.com/alan/cherry-picker/cmd"
	"github.com/spf13/cobra"
)

// NewStatusCmd creates and returns the status command
func NewStatusCmd(globalConfigFile *string, loadConfig func(string) (*cmd.Config, error)) *cobra.Command {
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of tracked PRs across target branches",
		Long: `Display the current status of all tracked PRs.
Shows which PRs are pending, picked, or merged for each target branch.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runStatus(*globalConfigFile, loadConfig)
		},
	}

	return statusCmd
}

func runStatus(configFile string, loadConfig func(string) (*cmd.Config, error)) error {
	config, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(config.TrackedPRs) == 0 {
		fmt.Println("No PRs to track.")
		return nil
	}

	sortPRsByNumber(config.TrackedPRs)
	displayRepositoryHeader(config)
	displayAllPRStatuses(config.TrackedPRs, config, configFile)
	displayStatusSummary(config.TrackedPRs)

	return nil
}

// sortPRsByNumber sorts PRs by number for consistent output
func sortPRsByNumber(activePRs []cmd.TrackedPR) {
	sort.Slice(activePRs, func(i, j int) bool {
		return activePRs[i].Number < activePRs[j].Number
	})
}

// displayRepositoryHeader shows repository information
func displayRepositoryHeader(config *cmd.Config) {
	fmt.Printf("Cherry-pick status for %s/%s (source: %s)\n\n", config.Org, config.Repo, config.SourceBranch)
}

// displayAllPRStatuses displays the status of all PRs
func displayAllPRStatuses(prs []cmd.TrackedPR, config *cmd.Config, configFile string) {
	for _, pr := range prs {
		displayPRStatus(pr, config, configFile)
		fmt.Println()
	}
}

// displayPRStatus displays the status of a single PR across all branches
func displayPRStatus(pr cmd.TrackedPR, config *cmd.Config, configFile string) {
	displayPRHeader(pr, config)

	if len(pr.Branches) == 0 {
		fmt.Println("  No branch status recorded")
		return
	}

	displayTrackedBranches(pr.Branches, config, pr.Number, configFile)
}

// displayPRHeader shows the PR number, title, and URL
func displayPRHeader(pr cmd.TrackedPR, config *cmd.Config) {
	// Generate GitHub PR URL
	url := fmt.Sprintf("https://github.com/%s/%s/pull/%d", config.Org, config.Repo, pr.Number)

	// Display title and URL instead of just PR number
	if pr.Title != "" {
		fmt.Printf("%s (%s)", pr.Title, url)
	} else {
		fmt.Printf("%s", url)
	}

	fmt.Println()
}

// displayTrackedBranches shows status for all tracked branches
func displayTrackedBranches(branches map[string]cmd.BranchStatus, config *cmd.Config, prNumber int, configFile string) {
	sortedBranches := getSortedBranchNames(branches)
	for _, branch := range sortedBranches {
		status := branches[branch]
		displayBranchStatus(branch, status, config, prNumber, configFile)
	}
}

// getSortedBranchNames returns branch names sorted alphabetically
func getSortedBranchNames(branches map[string]cmd.BranchStatus) []string {
	var branchNames []string
	for branch := range branches {
		branchNames = append(branchNames, branch)
	}
	sort.Strings(branchNames)
	return branchNames
}

// ciStatusInfo holds display information for a CI status
type ciStatusInfo struct {
	indicator        string
	suggestedCommand string
}

// getCIStatusInfo returns display information for a given CI status
func getCIStatusInfo(ciStatus cmd.CIStatus, executablePath, configFlag string, prNumber int, branch string) ciStatusInfo {
	switch ciStatus {
	case cmd.CIStatusPassing:
		return ciStatusInfo{
			indicator:        "✅ CI passing",
			suggestedCommand: fmt.Sprintf("%s%s merge %d %s", executablePath, configFlag, prNumber, branch),
		}
	case cmd.CIStatusFailing:
		return ciStatusInfo{
			indicator:        "❌ CI failing",
			suggestedCommand: fmt.Sprintf("%s%s retry %d %s", executablePath, configFlag, prNumber, branch),
		}
	case cmd.CIStatusPending:
		return ciStatusInfo{
			indicator:        "🔄 CI pending",
			suggestedCommand: "", // No action needed while CI is running
		}
	case cmd.CIStatusUnknown:
		return ciStatusInfo{
			indicator:        "❓ CI unknown",
			suggestedCommand: "",
		}
	default:
		return ciStatusInfo{
			indicator:        "❓ CI " + string(ciStatus),
			suggestedCommand: "",
		}
	}
}

// displayBranchStatus displays the status for a single branch
func displayBranchStatus(branch string, status cmd.BranchStatus, config *cmd.Config, prNumber int, configFile string) {
	executablePath := os.Args[0]
	configFlag := getConfigFlag(configFile)

	switch status.Status {
	case cmd.BranchStatusPending:
		fmt.Printf("  %-15s: ⏳ pending (bot hasn't attempted)\n", branch)
	case cmd.BranchStatusFailed:
		fmt.Printf("  %-15s: ❌ failed (bot couldn't cherry-pick)\n", branch)
		// Show pick command for AI-assisted resolution
		fmt.Printf("  %-15s  💡 %s%s pick %d %s\n", "", executablePath, configFlag, prNumber, branch)
	case cmd.BranchStatusPicked:
		if status.PR != nil {
			prURL := fmt.Sprintf("https://github.com/%s/%s/pull/%d", config.Org, config.Repo, status.PR.Number)
			fmt.Printf("  %-15s: 🔄 picked (%s)\n", branch, prURL)

			// Show stored PR details underneath
			fmt.Printf("  %-15s  %s", "", status.PR.Title)

			// Get CI status display info
			ciInfo := getCIStatusInfo(status.PR.CIStatus, executablePath, configFlag, prNumber, branch)

			fmt.Printf(" [%s]", ciInfo.indicator)
			fmt.Println()

			// Show suggested command if available
			if ciInfo.suggestedCommand != "" {
				fmt.Printf("  %-15s  💡 %s\n", "", ciInfo.suggestedCommand)
			}
		} else {
			fmt.Printf("  %-15s: ✅ picked\n", branch)
		}
	case cmd.BranchStatusMerged:
		fmt.Printf("  %-15s: ✅ merged\n", branch)
	case cmd.BranchStatusReleased:
		fmt.Printf("  %-15s: 🎉 released\n", branch)
	default:
		fmt.Printf("  %-15s: ❓ unknown status: %s\n", branch, status.Status)
	}
}

// displayStatusSummary displays the summary statistics
func displayStatusSummary(prs []cmd.TrackedPR) {
	totalPending := 0
	totalFailed := 0
	totalPicked := 0
	totalMerged := 0
	totalReleased := 0
	for _, pr := range prs {
		for _, status := range pr.Branches {
			switch status.Status {
			case cmd.BranchStatusPending:
				totalPending++
			case cmd.BranchStatusFailed:
				totalFailed++
			case cmd.BranchStatusPicked:
				totalPicked++
			case cmd.BranchStatusMerged:
				totalMerged++
			case cmd.BranchStatusReleased:
				totalReleased++
			}
		}
	}

	totalCompleted := totalPicked + totalMerged + totalReleased
	fmt.Printf("Summary: %d PR(s), %d pending, %d failed, %d completed (%d picked, %d merged, %d released)\n",
		len(prs), totalPending, totalFailed, totalCompleted, totalPicked, totalMerged, totalReleased)
}

// getConfigFlag returns the config flag if not using default
func getConfigFlag(configFile string) string {
	if configFile == "cherry-picks.yaml" {
		return ""
	}
	return fmt.Sprintf(" --config %s", configFile)
}
