package commands

import (
	"fmt"
	"os/exec"
	"path/filepath"
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

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Extract the file path (skip the first 3 characters which are the status codes)
		if len(line) > 3 {
			filePath := strings.TrimSpace(line[3:])
			if !IsCherryPickerFile(filePath) {
				return false
			}
		}
	}
	return true
}

// IsCherryPickerFile checks if a file is a cherry-picker configuration file
func IsCherryPickerFile(filePath string) bool {
	fileName := filepath.Base(filePath)
	return fileName == "cherry-picks.yaml"
}
