package commands

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ValidateGitRepository ensures we're in a Git repository with a clean working
// directory, ignoring the tool's own files: the config file at configFile plus
// the .lock/.tmp siblings written next to it by internal/state, and a few local
// assets (see keepFiles). When the tree is dirty the error names the offending
// files so the cause is obvious.
func ValidateGitRepository(configFile string) error {
	if !IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	dirty, err := UncleanFiles(configFile)
	if err != nil {
		return fmt.Errorf("failed to check working directory status: %w", err)
	}
	if len(dirty) > 0 {
		return fmt.Errorf("working directory is not clean, please commit or stash these changes:\n  %s",
			strings.Join(dirty, "\n  "))
	}

	return nil
}

// IsGitRepository checks if the current directory is a git repository
func IsGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// UncleanFiles returns the working-tree paths that make the tree dirty,
// excluding the tool's own files. An empty result means the tree is clean.
func UncleanFiles(configFile string) ([]string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var dirty []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(output)), "\n") {
		if len(line) <= 3 {
			continue
		}
		// Skip the first 3 characters, which are the porcelain status codes.
		filePath := strings.TrimSpace(line[3:])
		if !isLocalFile(filePath, configFile) {
			dirty = append(dirty, filePath)
		}
	}
	return dirty, nil
}

// IsWorkingDirectoryClean reports whether the working tree is clean, ignoring
// the tool's own files.
func IsWorkingDirectoryClean(configFile string) bool {
	dirty, err := UncleanFiles(configFile)
	return err == nil && len(dirty) == 0
}

// keepFiles are working-tree paths that should not make the tree count as
// "dirty" for the pick command's pre-flight check: the tool's own state files
// and local Claude assets. Entries ending in "/" match any path under that
// directory; other entries match by exact path or basename, and their ".lock"
// sidecars are matched too. The active config file (which may be renamed via
// --config) is handled separately in isLocalFile.
var keepFiles = []string{
	"cherry-picker.yaml", // unified config+state file (default)
	"cherry-picks.yaml",  // legacy cherry-picker state (pre-migration)
	"dep-merger.yaml",    // legacy dep-merger state (pre-migration)
	"CLAUDE.md",
	".claude/",
}

// isLocalFile reports whether a working-tree path is one of the tool's own
// files and can be ignored when checking that the tree is clean. configFile is
// the active config path (possibly empty); its basename and .lock sibling are
// always ignored.
func isLocalFile(filePath, configFile string) bool {
	name := filepath.Base(filePath)

	// The active config file and its lock sidecar (internal/state writes
	// "<config>.lock" next to it).
	if configFile != "" {
		cfgName := filepath.Base(configFile)
		if name == cfgName || name == cfgName+".lock" {
			return true
		}
	}

	// Transient atomic-save temp files: ".cherry-picker-*.tmp".
	if strings.HasPrefix(name, ".cherry-picker-") && strings.HasSuffix(name, ".tmp") {
		return true
	}

	for _, keep := range keepFiles {
		if strings.HasSuffix(keep, "/") {
			if strings.HasPrefix(filePath, keep) {
				return true
			}
			continue
		}
		if filePath == keep || name == keep || name == keep+".lock" {
			return true
		}
	}
	return false
}
