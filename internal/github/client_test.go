package github

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	ctx := context.Background()
	token := "test-token"

	client := NewClient(ctx, token)

	require.NotNil(t, client)
	assert.NotNil(t, client.client)
	assert.Equal(t, ctx, client.ctx)
}

func TestWithRepository(t *testing.T) {
	ctx := context.Background()
	token := "test-token"

	originalClient := NewClient(ctx, token)

	tests := []struct {
		name string
		org  string
		repo string
	}{
		{
			name: "standard org and repo",
			org:  "testorg",
			repo: "testrepo",
		},
		{
			name: "empty org and repo",
			org:  "",
			repo: "",
		},
		{
			name: "org with special characters",
			org:  "test-org-123",
			repo: "test_repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newClient := originalClient.WithRepository(tt.org, tt.repo)

			require.NotNil(t, newClient)
			assert.Equal(t, tt.org, newClient.org)
			assert.Equal(t, tt.repo, newClient.repo)

			// Verify original client is unchanged
			assert.Empty(t, originalClient.org)
			assert.Empty(t, originalClient.repo)

			// Verify underlying client and context are shared
			assert.Equal(t, originalClient.client, newClient.client)
			assert.Equal(t, originalClient.ctx, newClient.ctx)
		})
	}
}

func TestWithRepository_ChainedCalls(t *testing.T) {
	ctx := context.Background()
	client := NewClient(ctx, "test-token")

	client1 := client.WithRepository("org1", "repo1")
	client2 := client.WithRepository("org2", "repo2")

	// Each call should create independent clients
	assert.Equal(t, "org1", client1.org)
	assert.Equal(t, "repo1", client1.repo)

	assert.Equal(t, "org2", client2.org)
	assert.Equal(t, "repo2", client2.repo)

	// Original should remain unchanged
	assert.Empty(t, client.org)
	assert.Empty(t, client.repo)
}

// Note: Integration tests for GetMergedPRs and GetPR would require a real GitHub token
// and network access, so we're keeping these as unit tests for the basic functionality.
// For integration testing, we would create separate test files or use build tags.
