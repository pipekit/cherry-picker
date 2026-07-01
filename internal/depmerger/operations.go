package depmerger

import (
	"context"
	"log/slog"
)

// PROperation represents a single PR operation
type PROperation func(ctx context.Context, pr *TrackedPR) error

// executeBulkPROperation runs an operation on all eligible PRs.
// Returns the count of successfully processed PRs.
func executeBulkPROperation(
	ctx context.Context,
	config *Config,
	requiredStatus CIStatus,
	operation PROperation,
	opName string,
) int {
	var count int
	for i := range config.TrackedPRs {
		pr := &config.TrackedPRs[i]
		if pr.Merged || pr.CIStatus != requiredStatus {
			continue
		}

		if err := operation(ctx, pr); err != nil {
			slog.Error("Failed to "+opName+" PR", "pr", pr.Number, "error", err)
			continue
		}
		count++
	}
	return count
}
