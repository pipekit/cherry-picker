package summary

import (
	"testing"

	"github.com/alan/cherry-picker/cmd"
)

func TestNewSummaryCmd(t *testing.T) {
	t.Run("creates command with correct use", func(t *testing.T) {
		configFile := "test-config.yaml"
		loadConfig := func(path string) (*cmd.Config, error) {
			return &cmd.Config{}, nil
		}

		cobraCmd := NewSummaryCmd(&configFile, loadConfig)

		if cobraCmd == nil {
			t.Fatal("NewSummaryCmd() returned nil")
		}

		if cobraCmd.Use != "summary <target-branch>" {
			t.Errorf("NewSummaryCmd().Use = %q, want %q", cobraCmd.Use, "summary <target-branch>")
		}
	})

	t.Run("creates command with correct short description", func(t *testing.T) {
		configFile := "test-config.yaml"
		loadConfig := func(path string) (*cmd.Config, error) {
			return &cmd.Config{}, nil
		}

		cobraCmd := NewSummaryCmd(&configFile, loadConfig)

		expectedShort := "Generate development progress summary for a branch"
		if cobraCmd.Short != expectedShort {
			t.Errorf("NewSummaryCmd().Short = %q, want %q", cobraCmd.Short, expectedShort)
		}
	})

	t.Run("requires exactly one argument", func(t *testing.T) {
		configFile := "test-config.yaml"
		loadConfig := func(path string) (*cmd.Config, error) {
			return &cmd.Config{}, nil
		}

		cobraCmd := NewSummaryCmd(&configFile, loadConfig)

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
		loadConfig := func(path string) (*cmd.Config, error) {
			return &cmd.Config{}, nil
		}

		cobraCmd := NewSummaryCmd(&configFile, loadConfig)

		if !cobraCmd.SilenceUsage {
			t.Error("NewSummaryCmd().SilenceUsage should be true")
		}
	})
}

func TestSummaryCommand_Structure(t *testing.T) {
	t.Run("has BaseCommand embedded", func(t *testing.T) {
		summaryCmd := &SummaryCommand{}

		// Check that we can access BaseCommand fields
		summaryCmd.ConfigFile = new(string)
		if summaryCmd.ConfigFile == nil {
			t.Error("SummaryCommand should have ConfigFile from BaseCommand")
		}
	})

	t.Run("has TargetBranch field", func(t *testing.T) {
		summaryCmd := &SummaryCommand{
			TargetBranch: "release-3.7",
		}

		if summaryCmd.TargetBranch != "release-3.7" {
			t.Errorf("SummaryCommand.TargetBranch = %q, want %q", summaryCmd.TargetBranch, "release-3.7")
		}
	})
}

func TestSummaryCommand_Run(t *testing.T) {
	t.Run("Run method exists and can be called", func(t *testing.T) {
		summaryCmd := &SummaryCommand{
			TargetBranch: "release-3.7",
		}

		// We can't easily test Run without a full setup, but we can verify it exists
		// and has the right signature by attempting to assign it
		var _ func() error = summaryCmd.Run
	})
}
