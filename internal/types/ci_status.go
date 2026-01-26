// Package types provides shared types used across cherry-picker and dep-merger.
package types

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
