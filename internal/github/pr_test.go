package github

import (
	"testing"

	"github.com/google/go-github/v57/github"
	"github.com/stretchr/testify/assert"
)

func TestExtractCherryPickBranchesFromLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   []*github.Label
		expected []string
	}{
		{
			name: "single cherry-pick label",
			labels: []*github.Label{
				{Name: stringPtr("cherry-pick/3.6")},
			},
			expected: []string{"release-3.6"},
		},
		{
			name: "multiple cherry-pick labels",
			labels: []*github.Label{
				{Name: stringPtr("cherry-pick/3.6")},
				{Name: stringPtr("cherry-pick/3.7")},
				{Name: stringPtr("cherry-pick/4.0")},
			},
			expected: []string{"release-3.6", "release-3.7", "release-4.0"},
		},
		{
			name: "mixed labels - only cherry-pick extracted",
			labels: []*github.Label{
				{Name: stringPtr("bug")},
				{Name: stringPtr("cherry-pick/3.6")},
				{Name: stringPtr("enhancement")},
				{Name: stringPtr("cherry-pick/3.7")},
			},
			expected: []string{"release-3.6", "release-3.7"},
		},
		{
			name:     "no cherry-pick labels",
			labels:   []*github.Label{{Name: stringPtr("bug")}, {Name: stringPtr("enhancement")}},
			expected: nil,
		},
		{
			name:     "empty labels",
			labels:   []*github.Label{},
			expected: nil,
		},
		{
			name:     "nil labels",
			labels:   nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCherryPickBranchesFromLabels(tt.labels)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterCherryPickLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   []*github.Label
		expected []string
	}{
		{
			name: "filters cherry-pick labels",
			labels: []*github.Label{
				{Name: stringPtr("cherry-pick/3.6")},
				{Name: stringPtr("bug")},
				{Name: stringPtr("cherry-pick/3.7")},
			},
			expected: []string{"cherry-pick/3.6", "cherry-pick/3.7"},
		},
		{
			name:     "no cherry-pick labels returns empty",
			labels:   []*github.Label{{Name: stringPtr("bug")}},
			expected: nil, // Go idiom: nil slice is equivalent to empty slice
		},
		{
			name: "cherry-pick prefix matching",
			labels: []*github.Label{
				{Name: stringPtr("cherry-pick-beta")}, // Should match (starts with cherry-pick)
				{Name: stringPtr("not-cherry-pick")},  // Should not match
			},
			expected: []string{"cherry-pick-beta"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterCherryPickLabels(tt.labels)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildSearchQuery(t *testing.T) {
	tests := []struct {
		name     string
		org      string
		repo     string
		branch   string
		labels   []string
		expected string
	}{
		{
			name:     "single label",
			org:      "test-org",
			repo:     "test-repo",
			branch:   "main",
			labels:   []string{"cherry-pick/3.6"},
			expected: `repo:test-org/test-repo is:pr is:merged base:main label:cherry-pick/3.6`,
		},
		{
			name:     "multiple labels with comma (OR)",
			org:      "test-org",
			repo:     "test-repo",
			branch:   "main",
			labels:   []string{"cherry-pick/3.6", "cherry-pick/3.7"},
			expected: `repo:test-org/test-repo is:pr is:merged base:main label:cherry-pick/3.6,cherry-pick/3.7`,
		},
		{
			name:     "no labels",
			org:      "test-org",
			repo:     "test-repo",
			branch:   "main",
			labels:   []string{},
			expected: "repo:test-org/test-repo is:pr is:merged base:main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSearchQuery(tt.org, tt.repo, tt.branch, tt.labels)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractOrgFromIssue(t *testing.T) {
	tests := []struct {
		name     string
		issue    *github.Issue
		expected string
	}{
		{
			name: "valid issue with repository owner",
			issue: &github.Issue{
				Repository: &github.Repository{
					Owner: &github.User{Login: stringPtr("test-org")},
				},
			},
			expected: "test-org",
		},
		{
			name:     "nil repository",
			issue:    &github.Issue{Repository: nil},
			expected: "",
		},
		{
			name: "nil owner",
			issue: &github.Issue{
				Repository: &github.Repository{Owner: nil},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractOrgFromIssue(tt.issue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractRepoFromIssue(t *testing.T) {
	tests := []struct {
		name     string
		issue    *github.Issue
		expected string
	}{
		{
			name: "valid issue with repository",
			issue: &github.Issue{
				Repository: &github.Repository{Name: stringPtr("test-repo")},
			},
			expected: "test-repo",
		},
		{
			name:     "nil repository",
			issue:    &github.Issue{Repository: nil},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRepoFromIssue(tt.issue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
