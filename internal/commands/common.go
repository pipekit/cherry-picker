package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/github"
	"github.com/spf13/cobra"
)

// FindAndValidatePR finds a PR in the config and validates it exists
func FindAndValidatePR(config *cmd.Config, prNumber int) (*cmd.TrackedPR, error) {
	for i := range config.TrackedPRs {
		if config.TrackedPRs[i].Number == prNumber {
			return &config.TrackedPRs[i], nil
		}
	}
	return nil, fmt.Errorf("PR #%d not found in configuration", prNumber)
}

// DetermineBranchesToUpdate determines which branches need to be updated
func DetermineBranchesToUpdate(pr *cmd.TrackedPR, targetBranch string) []string {
	if targetBranch != "" {
		return []string{targetBranch}
	}

	var branches []string
	for branch := range pr.Branches {
		branches = append(branches, branch)
	}
	return branches
}

// ValidateTargetBranch validates that a target branch exists in the PR's branches
func ValidateTargetBranch(pr *cmd.TrackedPR, targetBranch string) error {
	if targetBranch == "" {
		return nil // Will operate on all branches
	}

	if _, exists := pr.Branches[targetBranch]; !exists {
		return fmt.Errorf("PR #%d has no status for branch '%s'", pr.Number, targetBranch)
	}
	return nil
}

// MessageFormatter handles success message formatting
type MessageFormatter struct{}

// FormatSuccessMessage creates a standardized success message
func (mf *MessageFormatter) FormatSuccessMessage(action string, prNumber int, targetBranch string, branches []string) string {
	var msg strings.Builder

	if targetBranch != "" {
		msg.WriteString(fmt.Sprintf("✅ Successfully %s PR #%d for branch %s\n", action, prNumber, targetBranch))
	} else {
		msg.WriteString(fmt.Sprintf("✅ Successfully %s PR #%d for %d branch(es): %s\n",
			action, prNumber, len(branches), strings.Join(branches, ", ")))
	}

	return msg.String()
}

// DisplaySuccessMessage displays a formatted success message
func DisplaySuccessMessage(action string, prNumber int, targetBranch string, branches []string) {
	mf := &MessageFormatter{}
	fmt.Print(mf.FormatSuccessMessage(action, prNumber, targetBranch, branches))
}

// Git repository validation functions

// ValidateGitRepository ensures we're in a Git repository with clean working directory
func ValidateGitRepository() error {
	if !IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	if !IsWorkingDirectoryClean() {
		return fmt.Errorf("working directory is not clean, please commit or stash your changes")
	}

	return nil
}

// IsGitRepository checks if the current directory is a git repository
func IsGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// IsWorkingDirectoryClean checks if the working directory is clean, ignoring cherry-picker files
func IsWorkingDirectoryClean() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Extract the file path (skip the first 3 characters which are the status codes)
		if len(line) > 3 {
			filePath := strings.TrimSpace(line[3:])
			if !IsCherryPickerFile(filePath) {
				return false
			}
		}
	}
	return true
}

// IsCherryPickerFile checks if a file is a cherry-picker configuration file
func IsCherryPickerFile(filePath string) bool {
	fileName := filepath.Base(filePath)
	return fileName == "cherry-picks.yaml"
}

// InitializeGitHubClient creates a GitHub client with proper token validation and repository context
func InitializeGitHubClient(config *cmd.Config) (*github.Client, context.Context, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, nil, fmt.Errorf("GITHUB_TOKEN environment variable is required")
	}

	ctx := context.Background()
	client := github.NewClient(ctx, token).WithRepository(config.Org, config.Repo)

	return client, ctx, nil
}

// PRCommandArgs holds parsed PR command arguments
type PRCommandArgs struct {
	PRNumber     int
	TargetBranch string
}

// ParsePRCommandArgs parses common PR command arguments (pr-number [target-branch])
func ParsePRCommandArgs(args []string) (*PRCommandArgs, error) {
	if len(args) == 0 {
		return &PRCommandArgs{}, nil // No arguments - operate on all
	}

	// Parse PR number
	prNumber, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("invalid PR number: %w", err)
	}

	// Parse target branch if provided
	var targetBranch string
	if len(args) > 1 {
		targetBranch = args[1]
	}

	return &PRCommandArgs{
		PRNumber:     prNumber,
		TargetBranch: targetBranch,
	}, nil
}

// BranchValidationPredicate defines a function that checks if a branch meets certain criteria
type BranchValidationPredicate func(branchStatus cmd.BranchStatus) bool

// ValidateBranchForOperation validates that a specific branch can be operated on
func ValidateBranchForOperation(trackedPR *cmd.TrackedPR, targetBranch string, operation string, predicate BranchValidationPredicate) error {
	// Check if branch exists and is picked
	branchStatus, exists := trackedPR.Branches[targetBranch]
	if !exists {
		return fmt.Errorf("branch %s is not tracked for PR #%d", targetBranch, trackedPR.Number)
	}

	if branchStatus.Status != cmd.BranchStatusPicked || branchStatus.PR == nil {
		return fmt.Errorf("PR #%d is not picked for branch %s", trackedPR.Number, targetBranch)
	}

	// Apply the specific validation predicate
	if !predicate(branchStatus) {
		return fmt.Errorf("PR #%d on branch %s does not meet requirements for %s", trackedPR.Number, targetBranch, operation)
	}

	return nil
}

// ValidateAnyBranchForOperation validates that at least one branch can be operated on
func ValidateAnyBranchForOperation(trackedPR *cmd.TrackedPR, operation string, predicate BranchValidationPredicate) error {
	hasEligibleBranch := false

	for _, branchStatus := range trackedPR.Branches {
		// Skip if not picked or no PR info
		if branchStatus.Status != cmd.BranchStatusPicked || branchStatus.PR == nil {
			continue
		}

		// Check if meets the criteria
		if predicate(branchStatus) {
			hasEligibleBranch = true
			break
		}
	}

	if !hasEligibleBranch {
		return fmt.Errorf("no picked branches meet requirements for %s for PR #%d", operation, trackedPR.Number)
	}

	return nil
}

// Common validation predicates

// IsEligibleForMerge checks if a branch is eligible for merging (CI passing, not already merged)
func IsEligibleForMerge(branchStatus cmd.BranchStatus) bool {
	return branchStatus.Status == cmd.BranchStatusPicked &&
		branchStatus.PR != nil &&
		branchStatus.PR.CIStatus == cmd.CIStatusPassing &&
		branchStatus.Status != cmd.BranchStatusMerged
}

// IsEligibleForRetry checks if a branch is eligible for CI retry (CI failing)
func IsEligibleForRetry(branchStatus cmd.BranchStatus) bool {
	return branchStatus.Status == cmd.BranchStatusPicked &&
		branchStatus.PR != nil &&
		branchStatus.PR.CIStatus == cmd.CIStatusFailing
}

// DisplayBulkOperationSuccess displays success messages for bulk operations (merge/retry all)
func DisplayBulkOperationSuccess(operation string, count int, errors []error, scope string) {
	if len(errors) > 0 {
		fmt.Printf("⚠️  Some %ss failed: %v\n", operation, errors)
	}

	if scope == "all" {
		fmt.Printf("✅ Successfully %s %d PR(s) across all tracked PRs\n", getOperationPastTense(operation), count)
	} else {
		fmt.Printf("✅ Successfully %s %d PR(s)\n", getOperationPastTense(operation), count)
	}
}

// getOperationPastTense returns the past tense form of operation verbs
func getOperationPastTense(operation string) string {
	switch operation {
	case "merge":
		return "merged"
	case "retry":
		return "triggered retry for"
	default:
		return operation + "d"
	}
}

// CommandBuilder helps create standardized PR commands
type CommandBuilder struct {
	Use          string
	Short        string
	Long         string
	MinArgs      int
	MaxArgs      int
	ConfigFlag   bool // Whether to add local --config flag
	ExampleUsage []string
}

// BuildCommand creates a cobra command with common patterns
func (cb *CommandBuilder) BuildCommand(runFunc func(cmd *cobra.Command, args []string) error) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:          cb.Use,
		Short:        cb.Short,
		Long:         cb.Long,
		Args:         cobra.RangeArgs(cb.MinArgs, cb.MaxArgs),
		SilenceUsage: true,
		RunE:         runFunc,
	}

	// Add examples if provided
	if len(cb.ExampleUsage) > 0 {
		examples := "\nExamples:\n"
		for _, example := range cb.ExampleUsage {
			examples += "  " + example + "\n"
		}
		cobraCmd.Long += examples
	}

	return cobraCmd
}

// ExecuteAllPattern encapsulates the common "execute on all eligible items" pattern
type ExecuteAllResult struct {
	TotalProcessed int
	Errors         []error
	OperationName  string
}

// HandleExecuteAllResult provides consistent messaging for bulk operations
func HandleExecuteAllResult(result *ExecuteAllResult, targetDescription string) error {
	if result.TotalProcessed == 0 {
		if len(result.Errors) > 0 {
			return fmt.Errorf("no operations completed due to errors: %v", result.Errors)
		}
		return fmt.Errorf("no eligible items found for %s", result.OperationName)
	}

	DisplayBulkOperationSuccess(result.OperationName, result.TotalProcessed, result.Errors, targetDescription)
	return nil
}

// ParsePRNumberFromArgs parses PR number from command arguments
func ParsePRNumberFromArgs(args []string, required bool) (int, error) {
	if len(args) == 0 {
		if required {
			return 0, fmt.Errorf("PR number is required")
		}
		return 0, nil
	}

	prNumber, err := strconv.Atoi(args[0])
	if err != nil {
		return 0, fmt.Errorf("invalid PR number: %w", err)
	}
	return prNumber, nil
}

// GetTargetBranchFromArgs extracts target branch from command arguments
func GetTargetBranchFromArgs(args []string) string {
	if len(args) > 1 {
		return args[1]
	}
	return ""
}

// BranchOperationFunc defines a function that operates on a single branch
type BranchOperationFunc func(client *github.Client, config *cmd.Config, trackedPR *cmd.TrackedPR, branchName string, branchStatus cmd.BranchStatus) error

// ExecuteOnAllEligibleBranches executes an operation on all eligible branches across all PRs
func ExecuteOnAllEligibleBranches(
	config *cmd.Config,
	operationName string,
	eligibilityPredicate BranchValidationPredicate,
	operation BranchOperationFunc,
	configFile string,
	saveConfig func(string, *cmd.Config) error,
	requiresConfigSave bool,
) error {
	// Initialize GitHub client
	client, _, err := InitializeGitHubClient(config)
	if err != nil {
		return err
	}

	var totalProcessed int
	var errors []error
	var configChanged bool

	fmt.Printf("🔍 Scanning all tracked PRs for %s operations...\n", operationName)

	// Iterate through all tracked PRs
	for prIndex := range config.TrackedPRs {
		trackedPR := &config.TrackedPRs[prIndex]
		prProcessedCount := 0

		// Check each branch for this PR
		for branchName, branchStatus := range trackedPR.Branches {
			// Skip if not eligible
			if !eligibilityPredicate(branchStatus) {
				continue
			}

			err := operation(client, config, trackedPR, branchName, branchStatus)
			if err != nil {
				errors = append(errors, fmt.Errorf("PR #%d branch %s: %w", trackedPR.Number, branchName, err))
				continue
			}

			prProcessedCount++
			totalProcessed++
			configChanged = true
		}

		if prProcessedCount > 0 {
			fmt.Printf("📊 Processed %d branch(es) for PR #%d\n", prProcessedCount, trackedPR.Number)
		}
	}

	// Save the updated configuration if any changes were made and required
	if requiresConfigSave && configChanged && saveConfig != nil {
		if err := saveConfig(configFile, config); err != nil {
			fmt.Printf("⚠️  Operations completed successfully but failed to update config: %v\n", err)
		}
	}

	result := &ExecuteAllResult{
		TotalProcessed: totalProcessed,
		Errors:         errors,
		OperationName:  operationName,
	}

	return HandleExecuteAllResult(result, "all")
}

// ExecuteOnEligibleBranchesForPR executes an operation on all eligible branches for a specific PR
func ExecuteOnEligibleBranchesForPR(
	trackedPR *cmd.TrackedPR,
	operationName string,
	eligibilityPredicate BranchValidationPredicate,
	operation BranchOperationFunc,
	config *cmd.Config,
	configFile string,
	saveConfig func(string, *cmd.Config) error,
	requiresConfigSave bool,
) error {
	// Initialize GitHub client
	client, _, err := InitializeGitHubClient(config)
	if err != nil {
		return err
	}

	var processedCount int
	var errors []error
	var configChanged bool

	// Check each branch for this PR
	for branchName, branchStatus := range trackedPR.Branches {
		// Skip if not eligible
		if !eligibilityPredicate(branchStatus) {
			continue
		}

		err := operation(client, config, trackedPR, branchName, branchStatus)
		if err != nil {
			errors = append(errors, fmt.Errorf("branch %s: %w", branchName, err))
			continue
		}

		processedCount++
		configChanged = true
	}

	// Save the updated configuration if any changes were made and required
	if requiresConfigSave && configChanged && saveConfig != nil {
		if err := saveConfig(configFile, config); err != nil {
			fmt.Printf("⚠️  Operations completed successfully but failed to update config: %v\n", err)
		}
	}

	if processedCount == 0 {
		if len(errors) > 0 {
			return fmt.Errorf("no operations completed due to errors: %v", errors)
		}
		return fmt.Errorf("no eligible branches found for %s for PR #%d", operationName, trackedPR.Number)
	}

	DisplayBulkOperationSuccess(operationName, processedCount, errors, fmt.Sprintf("PR #%d", trackedPR.Number))
	return nil
}
