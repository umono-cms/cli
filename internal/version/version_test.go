package version

import "testing"

func TestDisplay(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"semantic version", "1.2.3", "v1.2.3"},
		{"already prefixed semantic version", "v1.2.3", "v1.2.3"},
		{"development version", "0.0.0-dev+abc123", "v0.0.0-dev+abc123"},
		{"non semantic version", "dev", "dev"},
		{"empty version", "", "v0.0.0-dev"},
	}

	original := Version
	t.Cleanup(func() {
		Version = original
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version = tt.version

			if got := Display(); got != tt.want {
				t.Fatalf("Display() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUsableVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"v1.2.3", true},
		{"1.2.3", true},
		{"", false},
		{"(devel)", false},
	}

	for _, tt := range tests {
		if got := usableVersion(tt.version); got != tt.want {
			t.Fatalf("usableVersion(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}
