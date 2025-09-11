package github

import (
	"context"
	"testing"
)

func TestNewClient(t *testing.T) {
	ctx := context.Background()
	token := "test-token"

	client := NewClient(ctx, token)

	if client == nil {
		t.Error("NewClient() returned nil")
	}

	if client.client == nil {
		t.Error("NewClient() client field is nil")
	}

	if client.ctx != ctx {
		t.Error("NewClient() context not set correctly")
	}
}

// Note: Integration tests for GetMergedPRs and GetPR would require a real GitHub token
// and network access, so we're keeping these as unit tests for the basic functionality.
// For integration testing, we would create separate test files or use build tags.
