// Package config provides functions for loading and saving cherry-picker configuration files.
package config

import (
	"fmt"
	"os"

	"github.com/alan/cherry-picker/cmd"
	"gopkg.in/yaml.v3"
)

// LoadConfig loads the configuration from the specified file
func LoadConfig(filename string) (*cmd.Config, error) {
	data, err := os.ReadFile(filename) //nolint:gosec // Config filename is from command-line flag
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config cmd.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// SaveConfig saves the configuration to the specified file
func SaveConfig(filename string, config *cmd.Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
