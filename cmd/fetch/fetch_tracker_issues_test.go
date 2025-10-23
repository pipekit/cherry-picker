package fetch

import (
	"strings"
	"testing"

	"github.com/alan/cherry-picker/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractVersion(t *testing.T) {
	t.Run("extracts version from release branch", func(t *testing.T) {
		version, ok := strings.CutPrefix("release-3.6", "release-")
		assert.True(t, ok)
		assert.Equal(t, "3.6", version)
	})

	t.Run("returns false for non-release branch", func(t *testing.T) {
		version, ok := strings.CutPrefix("main", "release-")
		assert.False(t, ok)
		assert.Equal(t, "main", version) // CutPrefix returns original string when prefix not found
	})
}

func TestGetAllBranches(t *testing.T) {
	t.Run("extracts unique branches from multiple PRs", func(t *testing.T) {
		config := &cmd.Config{
			TrackedPRs: []cmd.TrackedPR{
				{
					Number: 123,
					Branches: map[string]cmd.BranchStatus{
						"release-3.6": {Status: cmd.BranchStatusPending},
						"release-3.7": {Status: cmd.BranchStatusPicked},
					},
				},
				{
					Number: 456,
					Branches: map[string]cmd.BranchStatus{
						"release-3.6": {Status: cmd.BranchStatusMerged}, // duplicate
						"release-3.8": {Status: cmd.BranchStatusPending},
					},
				},
			},
		}

		branches := getAllBranches(config)

		require.Len(t, branches, 3)
		assert.Contains(t, branches, "release-3.6")
		assert.Contains(t, branches, "release-3.7")
		assert.Contains(t, branches, "release-3.8")
	})
}
