package commands

import (
	"fmt"
	"strconv"

	"github.com/alan/cherry-picker/cmd"
)

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
