package types

import "testing"

func TestIsCriticalCheck(t *testing.T) {
	tests := []struct {
		name  string
		check string
		want  bool
	}{
		{"UI", "UI", true},
		{"Lint", "Lint", true},
		{"Codegen", "Codegen", true},
		{"gomod2nix", "gomod2nix", true},
		{"argo-images family", "argo-images (argocli)", true},
		{"Build family", "Build (linux/amd64)", true},
		{"non-critical", "unit-tests", false},
		{"dco", "DCO", false},
		{"substring is not enough", "run-gomod2nix-check", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCriticalCheck(tt.check); got != tt.want {
				t.Errorf("IsCriticalCheck(%q) = %v, want %v", tt.check, got, tt.want)
			}
		})
	}
}
