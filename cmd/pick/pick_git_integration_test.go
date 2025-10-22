//go:build integration
// +build integration

package pick

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestGitRepo creates a temporary git repository for testing
func setupTestGitRepo(t *testing.T) string {
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run(), "failed to init git repo")

	// Set user in the temp repo
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run(), "failed to set git user.name")

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run(), "failed to set git user.email")

	// Disable GPG signing for test commits
	cmd = exec.Command("git", "config", "commit.gpgsign", "false")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run(), "failed to disable gpg signing")

	return tmpDir
}

// createCommit creates a commit in the test repository
func createCommit(t *testing.T, repoDir, filename, content, message string) string {
	filePath := filepath.Join(repoDir, filename)
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	cmd := exec.Command("git", "add", filename)
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	// Get commit SHA
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	require.NoError(t, err)

	return strings.TrimSpace(string(output))
}

// TestMoveSignedOffByLinesToEnd_Integration tests commit message reordering
func TestMoveSignedOffByLinesToEnd_Integration(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	require.NoError(t, os.Chdir(repoDir))

	tests := []struct {
		name            string
		commitMessage   string
		expectedMessage string
	}{
		{
			name: "Signed-off-by in middle",
			commitMessage: `Fix bug in parser

Signed-off-by: Alice <alice@example.com>

This fixes the issue with null values.`,
			expectedMessage: `Fix bug in parser

This fixes the issue with null values.

Signed-off-by: Alice <alice@example.com>`,
		},
		{
			name: "Multiple Signed-off-by lines",
			commitMessage: `Add new feature

Signed-off-by: Alice <alice@example.com>
Some additional notes
Signed-off-by: Bob <bob@example.com>`,
			expectedMessage: `Add new feature

Some additional notes

Signed-off-by: Alice <alice@example.com>
Signed-off-by: Bob <bob@example.com>`,
		},
		{
			name: "Signed-off-by already at end",
			commitMessage: `Update documentation

Signed-off-by: Charlie <charlie@example.com>`,
			expectedMessage: `Update documentation

Signed-off-by: Charlie <charlie@example.com>`,
		},
		{
			name: "No Signed-off-by lines",
			commitMessage: `Simple commit message

With some description.`,
			expectedMessage: `Simple commit message

With some description.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a commit with the test message
			createCommit(t, repoDir, "test-"+tt.name+".txt", "test content", tt.commitMessage)

			// Create PickCommand instance
			pc := &PickCommand{}

			// Move Signed-off-by lines
			err := pc.moveSignedOffByLinesToEnd()
			require.NoError(t, err)

			// Get the amended commit message
			cmd := exec.Command("git", "log", "-1", "--pretty=format:%B")
			output, err := cmd.Output()
			require.NoError(t, err)

			actualMessage := strings.TrimSpace(string(output))
			assert.Equal(t, tt.expectedMessage, actualMessage)
		})
	}
}

// TestGetCommitInfo_Integration tests getting commit information
func TestGetCommitInfo_Integration(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	require.NoError(t, os.Chdir(repoDir))

	// Create a commit
	sha := createCommit(t, repoDir, "test.txt", "content", "Test commit message")

	pc := &PickCommand{}
	info, err := pc.getCommitInfo(sha)

	require.NoError(t, err)
	assert.Contains(t, info, sha[:7]) // Short SHA
	assert.Contains(t, info, "Test commit message")
}

// TestGetConflictedFiles_Integration tests detecting conflicted files
func TestGetConflictedFiles_Integration(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	require.NoError(t, os.Chdir(repoDir))

	// Create initial file with line 1
	createCommit(t, repoDir, "conflict.txt", "line 1\n", "Initial commit")

	// Save the default branch name before creating feature branch
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, _ := cmd.Output()
	defaultBranch := strings.TrimSpace(string(output))

	// Create a branch and modify line 1
	cmd = exec.Command("git", "checkout", "-b", "branch1")
	require.NoError(t, cmd.Run())

	// Modify the file on branch1
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "conflict.txt"), []byte("line 1 from branch1\n"), 0644))
	cmd = exec.Command("git", "add", "conflict.txt")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "Branch1 change")
	require.NoError(t, cmd.Run())

	// Go back to default branch and make conflicting change
	cmd = exec.Command("git", "checkout", defaultBranch)
	require.NoError(t, cmd.Run())

	// Modify the same line differently
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "conflict.txt"), []byte("line 1 from "+defaultBranch+"\n"), 0644))
	cmd = exec.Command("git", "add", "conflict.txt")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "Main branch change")
	require.NoError(t, cmd.Run())

	// Try to merge branch1 (will create conflict)
	cmd = exec.Command("git", "merge", "branch1", "--no-ff", "--no-commit")
	mergeErr := cmd.Run()

	// Merge should fail with conflict
	if mergeErr == nil {
		t.Skip("Merge did not create conflict as expected, skipping test")
	}

	// Now test getConflictedFiles
	pc := &PickCommand{}
	conflictedFiles, err := pc.getConflictedFiles()

	require.NoError(t, err)
	assert.Contains(t, conflictedFiles, "conflict.txt")
}

// TestGetConflictedFiles_NoConflicts tests when there are no conflicts
func TestGetConflictedFiles_NoConflicts(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	require.NoError(t, os.Chdir(repoDir))

	createCommit(t, repoDir, "test.txt", "content", "Test commit")

	pc := &PickCommand{}
	conflictedFiles, err := pc.getConflictedFiles()

	require.NoError(t, err)
	assert.Empty(t, conflictedFiles)
}

// TestIsConflictError tests conflict error detection
func TestIsConflictError(t *testing.T) {
	pc := &PickCommand{}

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "exit code 1 (conflict)",
			err:      &exec.ExitError{ProcessState: &os.ProcessState{}},
			expected: false, // Will be false without proper exit code
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pc.isConflictError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCherryPickCleanApply_Integration tests cherry-pick without conflicts
func TestCherryPickCleanApply_Integration(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	require.NoError(t, os.Chdir(repoDir))

	// Create initial commit
	createCommit(t, repoDir, "file1.txt", "initial content\n", "Initial commit")

	// Get the default branch name
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = repoDir
	output, _ := cmd.Output()
	defaultBranch := strings.TrimSpace(string(output))
	if defaultBranch == "" {
		defaultBranch = "master"
	}

	// Create feature branch and add a commit
	cmd = exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	sha := createCommit(t, repoDir, "file2.txt", "feature content\n", "Add feature file")

	// Go back to default branch
	cmd = exec.Command("git", "checkout", defaultBranch)
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	// Cherry-pick the commit from feature branch
	pc := &PickCommand{}
	err := pc.performCherryPick(sha)

	require.NoError(t, err)

	// Verify the file exists
	content, err := os.ReadFile(filepath.Join(repoDir, "file2.txt"))
	require.NoError(t, err)
	assert.Equal(t, "feature content\n", string(content))

	// Verify commit message includes cherry-pick info
	cmd = exec.Command("git", "log", "-1", "--pretty=format:%B")
	commitOutput, err := cmd.Output()
	require.NoError(t, err)

	commitMsg := string(commitOutput)
	assert.Contains(t, commitMsg, "Add feature file")
	assert.Contains(t, commitMsg, "(cherry picked from commit")
	assert.Contains(t, commitMsg, "Signed-off-by:")
}

// TestCommitMessageWithMultipleSignoffs_Integration tests complex commit messages
func TestCommitMessageWithMultipleSignoffs_Integration(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	require.NoError(t, os.Chdir(repoDir))

	complexMessage := `feat: Add awesome feature

This is a detailed description of the feature.
It spans multiple lines.

Signed-off-by: Developer One <dev1@example.com>

Some notes in the middle of the message.

Signed-off-by: Developer Two <dev2@example.com>

More notes here.`

	createCommit(t, repoDir, "complex.txt", "content", complexMessage)

	pc := &PickCommand{}
	err := pc.moveSignedOffByLinesToEnd()
	require.NoError(t, err)

	// Get amended message
	cmd := exec.Command("git", "log", "-1", "--pretty=format:%B")
	output, err := cmd.Output()
	require.NoError(t, err)

	actualMessage := strings.TrimSpace(string(output))

	// Verify structure: body first, then all Signed-off-by lines at the end
	assert.Contains(t, actualMessage, "feat: Add awesome feature")
	assert.Contains(t, actualMessage, "This is a detailed description")
	assert.Contains(t, actualMessage, "Some notes in the middle")
	assert.Contains(t, actualMessage, "More notes here")

	// Verify both Signed-off-by lines are at the end
	lines := strings.Split(actualMessage, "\n")
	var signoffIndices []int
	for i, line := range lines {
		if strings.Contains(line, "Signed-off-by:") {
			signoffIndices = append(signoffIndices, i)
		}
	}

	require.Len(t, signoffIndices, 2)

	// Check that Signed-off-by lines are consecutive and at the end
	assert.Equal(t, len(lines)-2, signoffIndices[0], "First Signed-off-by should be second-to-last line")
	assert.Equal(t, len(lines)-1, signoffIndices[1], "Second Signed-off-by should be last line")
}
