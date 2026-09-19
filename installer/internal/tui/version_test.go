package tui

import "testing"

func TestVersionLabel(t *testing.T) {
	original := Version
	defer func() { Version = original }()

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"default dev", "dev", "dev build"},
		{"empty", "", "dev build"},
		{"DEV in capitals", "DEV", "dev build"},
		{"plain release", "0.1.0", "v0.1.0"},
		{"already prefixed", "v0.1.0", "v0.1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version = tt.version
			if got := VersionLabel(); got != tt.want {
				t.Errorf("VersionLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
