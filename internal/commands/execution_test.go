package commands

import (
	"errors"
	"strings"
	"testing"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/github"
	"github.com/spf13/cobra"
)

func TestHandleExecuteAllResult(t *testing.T) {
	tests := []struct {
		name              string
		result            *ExecuteAllResult
		targetDescription string
		wantErr           bool
		wantErrorContains string
	}{
		{
			name: "successful execution",
			result: &ExecuteAllResult{
				TotalProcessed: 2,
				Errors:         []error{},
				OperationName:  "merge",
			},
			targetDescription: "all",
			wantErr:           false,
		},
		{
			name: "no eligible items",
			result: &ExecuteAllResult{
				TotalProcessed: 0,
				Errors:         []error{},
				OperationName:  "merge",
			},
			targetDescription: "all",
			wantErr:           true,
			wantErrorContains: "no eligible items found",
		},
		{
			name: "no operations completed due to errors",
			result: &ExecuteAllResult{
				TotalProcessed: 0,
				Errors:         []error{errors.New("test error")},
				OperationName:  "merge",
			},
			targetDescription: "all",
			wantErr:           true,
			wantErrorContains: "no operations completed due to errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := HandleExecuteAllResult(tt.result, tt.targetDescription)
			if (err != nil) != tt.wantErr {
				t.Errorf("HandleExecuteAllResult() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.wantErrorContains != "" && !strings.Contains(err.Error(), tt.wantErrorContains) {
				t.Errorf("HandleExecuteAllResult() error = %v, want error containing %v", err, tt.wantErrorContains)
			}
		})
	}
}

func TestCommandBuilder_BuildCommand(t *testing.T) {
	builder := &CommandBuilder{
		Use:          "test",
		Short:        "Test command",
		Long:         "This is a test command",
		MinArgs:      1,
		MaxArgs:      2,
		ConfigFlag:   true,
		ExampleUsage: []string{"test 123", "test 456 branch"},
	}

	runFunc := func(cmd *cobra.Command, args []string) error {
		return nil
	}

	cmd := builder.BuildCommand(runFunc)

	if cmd.Use != "test" {
		t.Errorf("BuildCommand() Use = %v, want %v", cmd.Use, "test")
	}

	if cmd.Short != "Test command" {
		t.Errorf("BuildCommand() Short = %v, want %v", cmd.Short, "Test command")
	}

	if !strings.Contains(cmd.Long, "Examples:") {
		t.Errorf("BuildCommand() Long should contain examples, got %v", cmd.Long)
	}

	if !strings.Contains(cmd.Long, "test 123") {
		t.Errorf("BuildCommand() Long should contain example usage, got %v", cmd.Long)
	}

	if !cmd.SilenceUsage {
		t.Errorf("BuildCommand() SilenceUsage should be true")
	}
}

func TestCommandBuilder_BuildCommandWithoutExamples(t *testing.T) {
	builder := &CommandBuilder{
		Use:     "test",
		Short:   "Test command",
		Long:    "This is a test command",
		MinArgs: 0,
		MaxArgs: 1,
	}

	runFunc := func(cmd *cobra.Command, args []string) error {
		return nil
	}

	cmd := builder.BuildCommand(runFunc)

	if strings.Contains(cmd.Long, "Examples:") {
		t.Errorf("BuildCommand() Long should not contain examples when none provided, got %v", cmd.Long)
	}
}

// Test helper functions for execution patterns (minimal testing since they require GitHub client)
func TestBranchOperationFuncSignature(t *testing.T) {
	// This test ensures the BranchOperationFunc type signature is correct
	var _ BranchOperationFunc = func(client *github.Client, config *cmd.Config, trackedPR *cmd.TrackedPR, branchName string, branchStatus cmd.BranchStatus) error {
		return nil
	}
}

func TestExecuteAllResultStruct(t *testing.T) {
	result := &ExecuteAllResult{
		TotalProcessed: 5,
		Errors:         []error{errors.New("test error")},
		OperationName:  "merge",
	}

	if result.TotalProcessed != 5 {
		t.Errorf("ExecuteAllResult.TotalProcessed = %v, want %v", result.TotalProcessed, 5)
	}

	if len(result.Errors) != 1 {
		t.Errorf("ExecuteAllResult.Errors length = %v, want %v", len(result.Errors), 1)
	}

	if result.OperationName != "merge" {
		t.Errorf("ExecuteAllResult.OperationName = %v, want %v", result.OperationName, "merge")
	}
}
