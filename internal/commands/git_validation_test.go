package commands

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsCherryPickerFile(t *testing.T) {
	tests := []struct {
		name       string
		filePath   string
		configFile string
		want       bool
	}{
		{
			name:     "unified cherry-picker.yaml file",
			filePath: "cherry-picker.yaml",
			want:     true,
		},
		{
			name:     "unified state lock sidecar",
			filePath: "cherry-picker.yaml.lock",
			want:     true,
		},
		{
			name:     "atomic-save temp file",
			filePath: ".cherry-picker-123.tmp",
			want:     true,
		},
		{
			name:     "cherry-picks.yaml file",
			filePath: "cherry-picks.yaml",
			want:     true,
		},
		{
			name:     "file under .claude/",
			filePath: ".claude/settings.json",
			want:     true,
		},
		{
			name:       "custom config path via --config",
			filePath:   "my-picks.yaml",
			configFile: "my-picks.yaml",
			want:       true,
		},
		{
			name:       "custom config lock sidecar",
			filePath:   "my-picks.yaml.lock",
			configFile: "my-picks.yaml",
			want:       true,
		},
		{
			name:     "other yaml file",
			filePath: "config.yaml",
			want:     false,
		},
		{
			name:     "go file",
			filePath: "main.go",
			want:     false,
		},
		{
			name:     "similar name but different",
			filePath: "cherry-picks.yml",
			want:     false,
		},
		{
			name:     "empty path",
			filePath: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLocalFile(tt.filePath, tt.configFile); got != tt.want {
				t.Errorf("isLocalFile(%q, %q) = %v, want %v", tt.filePath, tt.configFile, got, tt.want)
			}
		})
	}
}

func TestUncleanFilesIgnoresToolFilesInRepo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	require.NoError(t, exec.Command("git", "init").Run())

	// The tool's own files must be ignored...
	require.NoError(t, os.WriteFile("cherry-picker.yaml", []byte("org: x\n"), 0600))
	require.NoError(t, os.WriteFile("cherry-picker.yaml.lock", nil, 0600))
	require.NoError(t, os.WriteFile(".cherry-picker-42.tmp", nil, 0600))
	// ...but a genuinely untracked file must be reported by name.
	require.NoError(t, os.WriteFile("README.md", []byte("hi"), 0600))

	dirty, err := UncleanFiles("cherry-picker.yaml")
	require.NoError(t, err)
	assert.Equal(t, []string{"README.md"}, dirty)
	assert.False(t, IsWorkingDirectoryClean("cherry-picker.yaml"))

	require.NoError(t, os.Remove("README.md"))
	assert.True(t, IsWorkingDirectoryClean("cherry-picker.yaml"))
}
