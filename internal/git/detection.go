// Package git provides utilities for detecting git repository information.
package git

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// RepoInfo holds detected git repository information
type RepoInfo struct {
	Org          string
	Repo         string
	SourceBranch string // Optional: current branch when detected
}

// DetectRepoInfo attempts to detect git repository information
func DetectRepoInfo() (*RepoInfo, error) {
	if !IsGitRepository() {
		return nil, fmt.Errorf("not in a git repository")
	}

	org, repo, err := parseGitRemote()
	if err != nil {
		return nil, fmt.Errorf("failed to parse git remote: %w", err)
	}

	info := &RepoInfo{
		Org:  org,
		Repo: repo,
	}

	// Try to get current branch, but don't fail if we can't
	if branch, err := GetCurrentBranch(); err == nil {
		info.SourceBranch = branch
	}

	return info, nil
}

// IsGitRepository checks if current directory is in a git repository
func IsGitRepository() bool {
	gitCmd := exec.Command("git", "rev-parse", "--git-dir")
	return gitCmd.Run() == nil
}

// parseGitRemote extracts org and repo from git remote origin
func parseGitRemote() (string, string, error) {
	gitCmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := gitCmd.Output()
	if err != nil {
		return "", "", err
	}

	remoteURL := strings.TrimSpace(string(output))
	return ParseRemoteURL(remoteURL)
}

// ParseRemoteURL extracts org and repo from various GitHub URL formats
func ParseRemoteURL(remoteURL string) (string, string, error) {
	// Handle SSH format: git@github.com:org/repo.git
	sshRegex := regexp.MustCompile(`git@github\.com:([^/]+)/([^/]+?)(?:\.git)?$`)
	if matches := sshRegex.FindStringSubmatch(remoteURL); len(matches) == 3 {
		return matches[1], matches[2], nil
	}

	// Handle HTTPS format: https://github.com/org/repo.git
	httpsRegex := regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+?)(?:\.git)?/?$`)
	if matches := httpsRegex.FindStringSubmatch(remoteURL); len(matches) == 3 {
		return matches[1], matches[2], nil
	}

	return "", "", fmt.Errorf("unable to parse GitHub remote URL: %s", remoteURL)
}

// GetCurrentBranch gets the current git branch name
func GetCurrentBranch() (string, error) {
	gitCmd := exec.Command("git", "branch", "--show-current")
	output, err := gitCmd.Output()
	if err != nil {
		return "", err
	}

	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "", fmt.Errorf("unable to determine current branch")
	}

	return branch, nil
}
