// Package types provides shared types used across cherry-picker and dep-merger.
package types

import (
	"strings"

	"github.com/fatih/color"
)

// CIStatus represents the status of CI checks
type CIStatus string

const (
	// CIStatusPassing indicates all CI checks have passed
	CIStatusPassing CIStatus = "passing"
	// CIStatusFailing indicates one or more CI checks have failed
	CIStatusFailing CIStatus = "failing"
	// CIStatusPending indicates CI checks are still running
	CIStatusPending CIStatus = "pending"
	// CIStatusUnknown indicates CI status could not be determined
	CIStatusUnknown CIStatus = "unknown"
)

// ParseCIStatus converts a string to CIStatus
func ParseCIStatus(s string) CIStatus {
	switch s {
	case "passing":
		return CIStatusPassing
	case "failing":
		return CIStatusFailing
	case "pending":
		return CIStatusPending
	default:
		return CIStatusUnknown
	}
}

// IsCriticalCheck returns true if the check name matches patterns that indicate
// a critical failure: UI, Lint, Codegen, gomod2nix, argo-images.*, Build.*
func IsCriticalCheck(name string) bool {
	switch name {
	case "UI", "Lint", "Codegen", "gomod2nix":
		return true
	}
	return strings.HasPrefix(name, "argo-images") || strings.HasPrefix(name, "Build")
}

// FormatFailingChecks formats a list of failing checks, highlighting critical ones in bright red.
func FormatFailingChecks(checks []string) string {
	red := color.New(color.FgHiRed).SprintFunc()

	formatted := make([]string, len(checks))
	for i, check := range checks {
		if IsCriticalCheck(check) {
			formatted[i] = red(check)
		} else {
			formatted[i] = check
		}
	}
	return strings.Join(formatted, ", ")
}
