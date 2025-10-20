package cmd

import (
	"testing"
)

func TestConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "config with all fields",
			config: Config{
				Org:          "testorg",
				Repo:         "testrepo",
				SourceBranch: "main",
			},
		},
		{
			name: "config with different source branch",
			config: Config{
				Org:          "testorg",
				Repo:         "testrepo",
				SourceBranch: "develop",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the config struct can be created and accessed
			if tt.config.Org == "" {
				t.Error("Config.Org should not be empty in test")
			}
			if tt.config.Repo == "" {
				t.Error("Config.Repo should not be empty in test")
			}
			if tt.config.SourceBranch == "" {
				t.Error("Config.SourceBranch should not be empty in test")
			}
		})
	}
}
