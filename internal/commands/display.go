package commands

import (
	"fmt"
	"strings"
)

// formatSuccessMessage creates a standardized success message
func formatSuccessMessage(action string, prNumber int, targetBranch string, branches []string) string {
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
	fmt.Print(formatSuccessMessage(action, prNumber, targetBranch, branches))
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
