package commands

import (
	"errors"
	"testing"
)

func TestMessageFormatter_FormatSuccessMessage(t *testing.T) {
	mf := &MessageFormatter{}

	tests := []struct {
		name         string
		action       string
		prNumber     int
		targetBranch string
		branches     []string
		wantContains []string
	}{
		{
			name:         "single branch specified",
			action:       "merged",
			prNumber:     123,
			targetBranch: "release-3.6",
			branches:     []string{"release-3.6"},
			wantContains: []string{"✅", "Successfully merged", "PR #123", "branch release-3.6"},
		},
		{
			name:         "multiple branches",
			action:       "retried",
			prNumber:     456,
			targetBranch: "",
			branches:     []string{"release-3.6", "release-3.7"},
			wantContains: []string{"✅", "Successfully retried", "PR #456", "2 branch(es)", "release-3.6, release-3.7"},
		},
		{
			name:         "single branch in list",
			action:       "picked",
			prNumber:     789,
			targetBranch: "",
			branches:     []string{"main"},
			wantContains: []string{"✅", "Successfully picked", "PR #789", "1 branch(es)", "main"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mf.FormatSuccessMessage(tt.action, tt.prNumber, tt.targetBranch, tt.branches)

			for _, expectedSubstring := range tt.wantContains {
				if !contains(result, expectedSubstring) {
					t.Errorf("FormatSuccessMessage() = %v, want to contain %v", result, expectedSubstring)
				}
			}
		})
	}
}

func TestGetOperationPastTense(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		want      string
	}{
		{
			name:      "merge operation",
			operation: "merge",
			want:      "merged",
		},
		{
			name:      "retry operation",
			operation: "retry",
			want:      "triggered retry for",
		},
		{
			name:      "pick operation",
			operation: "pick",
			want:      "pickd",
		},
		{
			name:      "unknown operation",
			operation: "custom",
			want:      "customd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getOperationPastTense(tt.operation); got != tt.want {
				t.Errorf("getOperationPastTense() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(substr) <= len(s) && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test DisplayBulkOperationSuccess (testing the logic, not the actual output)
func TestDisplayBulkOperationSuccess_Logic(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		count     int
		errors    []error
		scope     string
	}{
		{
			name:      "successful bulk operation",
			operation: "merge",
			count:     3,
			errors:    []error{},
			scope:     "all",
		},
		{
			name:      "bulk operation with errors",
			operation: "retry",
			count:     2,
			errors:    []error{errors.New("failed to retry PR #123")},
			scope:     "PR #456",
		},
		{
			name:      "no operations completed",
			operation: "merge",
			count:     0,
			errors:    []error{},
			scope:     "all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test just verifies the function can be called without panicking
			// In a real application, you might want to capture stdout to test the actual output
			DisplayBulkOperationSuccess(tt.operation, tt.count, tt.errors, tt.scope)
		})
	}
}

// Test DisplaySuccessMessage (testing the logic, not the actual output)
func TestDisplaySuccessMessage_Logic(t *testing.T) {
	tests := []struct {
		name         string
		action       string
		prNumber     int
		targetBranch string
		branches     []string
	}{
		{
			name:         "display single branch message",
			action:       "merged",
			prNumber:     123,
			targetBranch: "release-3.6",
			branches:     []string{"release-3.6"},
		},
		{
			name:         "display multiple branches message",
			action:       "retried",
			prNumber:     456,
			targetBranch: "",
			branches:     []string{"release-3.6", "release-3.7"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test just verifies the function can be called without panicking
			// In a real application, you might want to capture stdout to test the actual output
			DisplaySuccessMessage(tt.action, tt.prNumber, tt.targetBranch, tt.branches)
		})
	}
}
