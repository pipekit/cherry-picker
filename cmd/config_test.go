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
				Org:            "testorg",
				Repo:           "testrepo",
				SourceBranch:   "main",
				TargetBranches: []string{"release-1.0", "release-2.0"},
			},
		},
		{
			name: "config with empty target branches",
			config: Config{
				Org:            "testorg",
				Repo:           "testrepo",
				SourceBranch:   "develop",
				TargetBranches: []string{},
			},
		},
		{
			name: "config with nil target branches",
			config: Config{
				Org:            "testorg",
				Repo:           "testrepo",
				SourceBranch:   "main",
				TargetBranches: nil,
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
			// TargetBranches can be empty or nil, so we just check it's accessible
			_ = tt.config.TargetBranches
		})
	}
}
