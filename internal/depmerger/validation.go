package depmerger

import "fmt"

// validateTrackedPR validates that a PR is tracked and returns it
func validateTrackedPR(config *Config, prNumber int) (*TrackedPR, error) {
	pr := FindTrackedPR(config, prNumber)
	if pr == nil {
		return nil, fmt.Errorf("PR #%d is not tracked (run 'fetch' first)", prNumber)
	}
	return pr, nil
}

// validatePRNotMerged validates that a PR is not already merged
func validatePRNotMerged(pr *TrackedPR) error {
	if pr.Merged {
		return fmt.Errorf("PR #%d is already merged", pr.Number)
	}
	return nil
}

// validatePRCIStatus validates that a PR has the expected CI status
func validatePRCIStatus(pr *TrackedPR, expectedStatus CIStatus) error {
	if pr.CIStatus != expectedStatus {
		return fmt.Errorf("PR #%d does not have %s CI (status: %s)", pr.Number, expectedStatus, pr.CIStatus)
	}
	return nil
}

// validatePRForOperation performs all validations for a PR operation
func validatePRForOperation(config *Config, prNumber int, expectedStatus CIStatus, _ string) (*TrackedPR, error) {
	pr, err := validateTrackedPR(config, prNumber)
	if err != nil {
		return nil, err
	}

	if err := validatePRNotMerged(pr); err != nil {
		return nil, err
	}

	if err := validatePRCIStatus(pr, expectedStatus); err != nil {
		return nil, err
	}

	return pr, nil
}
