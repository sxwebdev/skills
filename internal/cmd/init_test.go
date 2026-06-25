package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runInitArgs(t *testing.T, args ...string) error {
	t.Helper()
	return InitCmd().Run(t.Context(), append([]string{"init"}, args...))
}

func TestRunInit(t *testing.T) {
	t.Run("default name scaffolds skills/my-skill", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		if err := runInitArgs(t); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "skills", "my-skill", "SKILL.md"))
		if err != nil {
			t.Fatalf("SKILL.md not created: %v", err)
		}
		if !strings.Contains(string(data), "name: my-skill") {
			t.Errorf("template missing name:\n%s", data)
		}
	})

	t.Run("custom name", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		if err := runInitArgs(t, "cool-skill"); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "skills", "cool-skill", "SKILL.md")); err != nil {
			t.Errorf("custom skill not created: %v", err)
		}
	})

	t.Run("refuses to overwrite without --force", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		if err := runInitArgs(t, "dup"); err != nil {
			t.Fatal(err)
		}
		err := runInitArgs(t, "dup")
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Errorf("err = %v, want already-exists error", err)
		}
	})

	t.Run("--force overwrites", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		if err := runInitArgs(t, "dup"); err != nil {
			t.Fatal(err)
		}
		// Mark the file so we can prove it was rewritten.
		path := filepath.Join(dir, "skills", "dup", "SKILL.md")
		if err := os.WriteFile(path, []byte("STALE"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runInitArgs(t, "dup", "--force"); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), "STALE") {
			t.Error("--force did not overwrite the file")
		}
	})

	t.Run("invalid name is rejected", func(t *testing.T) {
		t.Chdir(t.TempDir())
		err := runInitArgs(t, "..")
		if err == nil {
			t.Fatal("expected sanitize error for '..'")
		}
	})
}
