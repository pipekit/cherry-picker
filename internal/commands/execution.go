package commands

import (
	"context"
	"fmt"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/github"
	"github.com/spf13/cobra"
)

// BranchOperationFunc defines a function that operates on a single branch
type BranchOperationFunc func(ctx context.Context, client *github.Client, config *cmd.Config, trackedPR *cmd.TrackedPR, branchName string, branchStatus cmd.BranchStatus) error

// ExecuteAllResult encapsulates the result of bulk operations
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

// ExecuteOnAllEligibleBranches executes an operation on all eligible branches across all PRs
func ExecuteOnAllEligibleBranches(
	ctx context.Context,
	config *cmd.Config,
	operationName string,
	eligibilityPredicate BranchValidationPredicate,
	operation BranchOperationFunc,
	configFile string,
	saveConfig func(string, *cmd.Config) error,
	requiresConfigSave bool,
) error {
	// Initialize GitHub client
	client, _, err := InitializeGitHubClient(ctx, config)
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

			err := operation(ctx, client, config, trackedPR, branchName, branchStatus)
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
	ctx context.Context,
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
	client, _, err := InitializeGitHubClient(ctx, config)
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

		err := operation(ctx, client, config, trackedPR, branchName, branchStatus)
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
func (cb *CommandBuilder) BuildCommand(runFunc func(cobraCmd *cobra.Command, args []string) error) *cobra.Command {
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
