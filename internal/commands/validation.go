package commands

import (
	"fmt"

	"github.com/alan/cherry-picker/cmd"
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
