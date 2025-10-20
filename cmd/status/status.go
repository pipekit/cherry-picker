package status

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/alan/cherry-picker/cmd"
	"github.com/spf13/cobra"
)

// NewStatusCmd creates and returns the status command
func NewStatusCmd(globalConfigFile *string, loadConfig func(string) (*cmd.Config, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of tracked PRs across target branches",
		Long: `Display the current status of all tracked PRs.
Shows which PRs are pending, picked, or merged for each target branch.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(*globalConfigFile, loadConfig)
		},
	}

	return cmd
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

// displayBranchStatus displays the status for a single branch
func displayBranchStatus(branch string, status cmd.BranchStatus, config *cmd.Config, prNumber int, configFile string) {
	executablePath := os.Args[0]
	configFlag := getConfigFlag(configFile)

	switch status.Status {
	case "pending":
		fmt.Printf("  %-15s: ⏳ pending (bot hasn't attempted)\n", branch)
	case "failed":
		fmt.Printf("  %-15s: ❌ failed (bot couldn't cherry-pick)\n", branch)
		// Show pick command for AI-assisted resolution
		fmt.Printf("  %-15s  💡 %s%s pick %d %s\n", "", executablePath, configFlag, prNumber, branch)
	case "picked":
		if status.PR != nil {
			prURL := fmt.Sprintf("https://github.com/%s/%s/pull/%d", config.Org, config.Repo, status.PR.Number)
			fmt.Printf("  %-15s: 🔄 picked (%s)\n", branch, prURL)

			// Show stored PR details underneath
			fmt.Printf("  %-15s  %s", "", status.PR.Title)
			var indicators []string
			// Show contextual command based on CI status
			var suggestedCommand string
			switch status.PR.CIStatus {
			case "passing":
				indicators = append(indicators, "✅ CI passing")
				// Ready to merge
				suggestedCommand = fmt.Sprintf("%s%s merge %d %s", executablePath, configFlag, prNumber, branch)
			case "failing":
				indicators = append(indicators, "❌ CI failing")
				// Suggest retry
				suggestedCommand = fmt.Sprintf("%s%s retry %d %s", executablePath, configFlag, prNumber, branch)
			case "pending":
				indicators = append(indicators, "🔄 CI pending")
				// No action needed while CI is running
			case "unknown":
				indicators = append(indicators, "❓ CI unknown")
			default:
				indicators = append(indicators, "❓ CI "+string(status.PR.CIStatus))
			}

			if len(indicators) > 0 {
				fmt.Printf(" [%s]", strings.Join(indicators, ", "))
			}
			fmt.Println()

			// Show suggested command if available
			if suggestedCommand != "" {
				fmt.Printf("  %-15s  💡 %s\n", "", suggestedCommand)
			}
		} else {
			fmt.Printf("  %-15s: ✅ picked\n", branch)
		}
	case "merged":
		fmt.Printf("  %-15s: ✅ merged\n", branch)
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
	for _, pr := range prs {
		for _, status := range pr.Branches {
			switch status.Status {
			case "pending":
				totalPending++
			case "failed":
				totalFailed++
			case "picked":
				totalPicked++
			case "merged":
				totalMerged++
			}
		}
	}

	totalCompleted := totalPicked + totalMerged
	fmt.Printf("Summary: %d PR(s), %d pending, %d failed, %d completed (%d picked, %d merged)\n",
		len(prs), totalPending, totalFailed, totalCompleted, totalPicked, totalMerged)
}

// getConfigFlag returns the config flag if not using default
func getConfigFlag(configFile string) string {
	if configFile == "cherry-picks.yaml" {
		return ""
	}
	return fmt.Sprintf(" --config %s", configFile)
}
