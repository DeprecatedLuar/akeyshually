package executor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get home dir: %v", err)
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~/foo", filepath.Join(home, "foo")},
		{"~/a/b/c", filepath.Join(home, "a/b/c")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~", "~"}, // no trailing slash — not expanded by design
		{"", ""},
	}

	for _, tt := range tests {
		got := expandHome(tt.input)
		if got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
