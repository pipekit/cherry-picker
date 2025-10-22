package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCIStatusChecker_IsDCOCheck(t *testing.T) {
	checker := &CIStatusChecker{
		dcoPatterns: []string{"dco", "DCO", "developer-certificate-of-origin", "signoff", "sign-off", "signed-off-by"},
	}

	tests := []struct {
		name      string
		checkName string
		expected  bool
	}{
		{name: "lowercase dco", checkName: "dco", expected: true},
		{name: "uppercase DCO", checkName: "DCO", expected: true},
		{name: "DCO check", checkName: "DCO Check", expected: true},
		{name: "dco/check", checkName: "dco/check", expected: true},
		{name: "developer-certificate-of-origin", checkName: "developer-certificate-of-origin", expected: true},
		{name: "signoff", checkName: "signoff", expected: true},
		{name: "sign-off", checkName: "sign-off", expected: true},
		{name: "signed-off-by", checkName: "Signed-off-by", expected: true},
		{name: "lint check", checkName: "lint", expected: false},
		{name: "test check", checkName: "test", expected: false},
		{name: "build", checkName: "build", expected: false},
		{name: "ci/circleci", checkName: "ci/circleci", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.isDCOCheck(tt.checkName)
			assert.Equal(t, tt.expected, result, "check name: %s", tt.checkName)
		})
	}
}

func TestCIStatusChecker_AggregateStatus(t *testing.T) {
	checker := &CIStatusChecker{}

	tests := []struct {
		name              string
		combinedStatus    string
		checkRunsStatus   string
		expectedAggregate string
	}{
		{
			name:              "both passing",
			combinedStatus:    "passing",
			checkRunsStatus:   "passing",
			expectedAggregate: "passing",
		},
		{
			name:              "combined pending takes priority",
			combinedStatus:    "pending",
			checkRunsStatus:   "passing",
			expectedAggregate: "pending",
		},
		{
			name:              "check runs pending takes priority",
			combinedStatus:    "passing",
			checkRunsStatus:   "pending",
			expectedAggregate: "pending",
		},
		{
			name:              "combined failing",
			combinedStatus:    "failing",
			checkRunsStatus:   "passing",
			expectedAggregate: "failing",
		},
		{
			name:              "check runs failing",
			combinedStatus:    "passing",
			checkRunsStatus:   "failing",
			expectedAggregate: "failing",
		},
		{
			name:              "pending overrides failing",
			combinedStatus:    "pending",
			checkRunsStatus:   "failing",
			expectedAggregate: "pending",
		},
		{
			name:              "both failing",
			combinedStatus:    "failing",
			checkRunsStatus:   "failing",
			expectedAggregate: "failing",
		},
		{
			name:              "unknown statuses default to combined",
			combinedStatus:    "unknown",
			checkRunsStatus:   "unknown",
			expectedAggregate: "unknown",
		},
		{
			name:              "mixed unknown defaults to combined",
			combinedStatus:    "unknown",
			checkRunsStatus:   "passing",
			expectedAggregate: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.aggregateStatus(tt.combinedStatus, tt.checkRunsStatus)
			assert.Equal(t, tt.expectedAggregate, result)
		})
	}
}

// Note: evaluateStatuses is tested indirectly through integration tests
// as it requires github.RepoStatus objects which are hard to mock
