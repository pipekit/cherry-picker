package state

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load reads and parses the unified state file. It takes no lock: Save writes
// atomically via os.Rename, so a concurrent reader always sees either the old
// or the new complete file, never a torn one.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is from command-line flag
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// Save writes the config atomically: marshal, write a temp file in the same
// directory, fsync it, then os.Rename over the destination. The rename is
// atomic on POSIX filesystems, so readers never observe a partial write.
func Save(path string, c *Config) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".cherry-picker-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if we fail before the rename.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0600); err != nil {
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("failed to rename temp file into place: %w", err)
	}

	// Best-effort fsync of the directory so the rename is durable.
	if d, err := os.Open(dir); err == nil { //nolint:gosec // dir is the config file's own directory
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
