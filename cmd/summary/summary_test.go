package summary

import (
	"testing"

	"github.com/alan/cherry-picker/cmd"
	"github.com/stretchr/testify/require"
)

func TestNewSummaryCmd(t *testing.T) {
	t.Run("creates command with correct use", func(t *testing.T) {
		configFile := "test-config.yaml"
		loadConfig := func(_ string) (*cmd.Config, error) {
			return &cmd.Config{}, nil
		}

		cobraCmd := NewSummaryCmd(&configFile, loadConfig)
		require.NotNil(t, cobraCmd)

		if cobraCmd.Use != "summary <target-branch>" {
			t.Errorf("NewSummaryCmd().Use = %q, want %q", cobraCmd.Use, "summary <target-branch>")
		}
	})

	t.Run("creates command with correct short description", func(t *testing.T) {
		configFile := "test-config.yaml"
		loadConfig := func(_ string) (*cmd.Config, error) {
			return &cmd.Config{}, nil
		}

		cobraCmd := NewSummaryCmd(&configFile, loadConfig)
		require.NotNil(t, cobraCmd)

		expectedShort := "Generate development progress summary for a branch"
		if cobraCmd.Short != expectedShort {
			t.Errorf("NewSummaryCmd().Short = %q, want %q", cobraCmd.Short, expectedShort)
		}
	})

	t.Run("requires exactly one argument", func(t *testing.T) {
		configFile := "test-config.yaml"
		loadConfig := func(_ string) (*cmd.Config, error) {
			return &cmd.Config{}, nil
		}

		cobraCmd := NewSummaryCmd(&configFile, loadConfig)
		require.NotNil(t, cobraCmd)

		// Test with no arguments
		err := cobraCmd.Args(cobraCmd, []string{})
		if err == nil {
			t.Error("NewSummaryCmd() should require at least one argument")
		}

		// Test with one argument
		err = cobraCmd.Args(cobraCmd, []string{"release-3.7"})
		if err != nil {
			t.Errorf("NewSummaryCmd() should accept one argument, got error: %v", err)
		}

		// Test with two arguments
		err = cobraCmd.Args(cobraCmd, []string{"release-3.7", "extra"})
		if err == nil {
			t.Error("NewSummaryCmd() should not accept more than one argument")
		}
	})

	t.Run("has SilenceUsage enabled", func(t *testing.T) {
		configFile := "test-config.yaml"
		loadConfig := func(_ string) (*cmd.Config, error) {
			return &cmd.Config{}, nil
		}

		cobraCmd := NewSummaryCmd(&configFile, loadConfig)
		require.NotNil(t, cobraCmd)

		if !cobraCmd.SilenceUsage {
			t.Error("NewSummaryCmd().SilenceUsage should be true")
		}
	})
}

func TestCommand_Structure(t *testing.T) {
	t.Run("has BaseCommand embedded", func(t *testing.T) {
		summaryCmd := &command{}

		// Check that we can access BaseCommand fields
		summaryCmd.ConfigFile = new(string)
		if summaryCmd.ConfigFile == nil {
			t.Error("command should have ConfigFile from BaseCommand")
		}
	})

	t.Run("has TargetBranch field", func(t *testing.T) {
		summaryCmd := &command{
			TargetBranch: "release-3.7",
		}

		if summaryCmd.TargetBranch != "release-3.7" {
			t.Errorf("command.TargetBranch = %q, want %q", summaryCmd.TargetBranch, "release-3.7")
		}
	})
}

func TestCommand_Run(t *testing.T) {
	t.Run("Run method exists and can be called", func(_ *testing.T) {
		summaryCmd := &command{
			TargetBranch: "release-3.7",
		}

		// We can't easily test Run without a full setup, but we can verify it exists
		// and has the right signature by attempting to assign it
		var _ = summaryCmd.Run
	})
}
