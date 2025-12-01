package commands

import (
	"testing"
)

func TestIsCherryPickerFile(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{
			name:     "cherry-picks.yaml file",
			filePath: "cherry-picks.yaml",
			want:     true,
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
			if got := isLocalFile(tt.filePath); got != tt.want {
				t.Errorf("IsCherryPickerFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
