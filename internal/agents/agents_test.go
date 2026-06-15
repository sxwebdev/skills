package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"claude-code", "claude-code", true},
		{"claude", "claude-code", true}, // alias
		{"cursor", "cursor", true},
		{"gemini-cli", "gemini", true}, // alias
		{"nonexistent", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			a, ok := Get(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("Get(%q) ok = %v, want %v", tt.in, ok, tt.wantOK)
			}
			if ok && a.Name != tt.want {
				t.Errorf("Get(%q).Name = %q, want %q", tt.in, a.Name, tt.want)
			}
		})
	}
}

func TestResolveDirs(t *testing.T) {
	t.Parallel()

	home, _ := os.UserHomeDir()

	if got, want := ResolveGlobalDir("claude-code"), filepath.Join(home, ".claude", "skills"); got != want {
		t.Errorf("ResolveGlobalDir(claude-code) = %q, want %q", got, want)
	}
	if got, want := ResolveProjectDir("cursor", "/proj"), filepath.Join("/proj", ".cursor", "skills"); got != want {
		t.Errorf("ResolveProjectDir(cursor) = %q, want %q", got, want)
	}
	if got := ResolveGlobalDir("unknown"); got != "" {
		t.Errorf("ResolveGlobalDir(unknown) = %q, want empty", got)
	}
}

func TestDetect(t *testing.T) {
	// Not parallel: mutates HOME via t.Setenv.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp) // Windows

	// No agent dirs yet → claude-code should not be detected via dir
	// (it may still be detected via PATH on a dev machine, so only assert
	// the positive case after creating the directory).
	if err := os.MkdirAll(filepath.Join(tmp, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, a := range Detect() {
		if a.Name == "cursor" {
			found = true
		}
	}
	if !found {
		t.Errorf("Detect() did not include cursor after creating ~/.cursor")
	}
}
