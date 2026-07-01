package commands

import (
	"fmt"
	"os/exec"
	"strings"
)

// ValidateGitRepository ensures we're in a Git repository with clean working directory
func ValidateGitRepository() error {
	if !IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	if !IsWorkingDirectoryClean() {
		return fmt.Errorf("working directory is not clean, please commit or stash your changes")
	}

	return nil
}

// IsGitRepository checks if the current directory is a git repository
func IsGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// IsWorkingDirectoryClean checks if the working directory is clean, ignoring cherry-picker files
func IsWorkingDirectoryClean() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	lines := strings.SplitSeq(strings.TrimSpace(string(output)), "\n")
	for line := range lines {
		if line == "" {
			continue
		}
		// Extract the file path (skip the first 3 characters which are the status codes)
		if len(line) > 3 {
			filePath := strings.TrimSpace(line[3:])
			if !isLocalFile(filePath) {
				return false
			}
		}
	}
	return true
}

// keepFiles are working-tree paths that should not make the tree count as
// "dirty" for the pick command's pre-flight check: the tool's own config/state
// files and local Claude assets. Entries ending in "/" match any path under
// that directory; other entries match exactly.
var keepFiles = []string{
	"cherry-picker.yaml", // unified config+state file (default)
	"cherry-picks.yaml",  // legacy cherry-picker state (pre-migration)
	"dep-merger.yaml",    // legacy dep-merger state (pre-migration)
	"CLAUDE.md",
	".claude/",
}

// isLocalFile reports whether a working-tree path is one of the tool's own
// files and can be ignored when checking that the tree is clean.
func isLocalFile(filePath string) bool {
	for _, keep := range keepFiles {
		if strings.HasSuffix(keep, "/") {
			if strings.HasPrefix(filePath, keep) {
				return true
			}
		} else if filePath == keep {
			return true
		}
	}
	return false
}
