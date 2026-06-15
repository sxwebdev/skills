package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMigratesV1ToV2(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	v1 := `{
  "version": 1,
  "repos": {"https://github.com/o/r.git": {"alias": "o/r", "added_at": "2024-01-01T00:00:00Z"}},
  "skills": {"my-skill": {"repo": "https://github.com/o/r.git", "path_in_repo": "skills/my-skill", "folder_hash": "abc", "installed_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z"}},
  "agents": ["claude-code"]
}`
	if err := os.MkdirAll(ConfigDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ConfigFile(), []byte(v1), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Version != SchemaVersion {
		t.Errorf("Version = %d, want %d", cfg.Version, SchemaVersion)
	}
	skill := cfg.Skills["my-skill"]
	if skill.Mode != ModeSymlink {
		t.Errorf("Mode = %q, want %q", skill.Mode, ModeSymlink)
	}
	if skill.HashKind != HashKindSHA1 {
		t.Errorf("HashKind = %q, want %q", skill.HashKind, HashKindSHA1)
	}
	if len(skill.Agents) != 1 || skill.Agents[0] != "claude-code" {
		t.Errorf("Agents = %v, want [claude-code]", skill.Agents)
	}

	// Migration is sticky: file on disk now reports v2.
	reloaded, err := os.ReadFile(ConfigFile())
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(reloaded), `"version": 2`) {
		t.Errorf("persisted config not upgraded to v2:\n%s", reloaded)
	}
}

func TestLoadOrCreateNoWriteWhenAbsent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	cfg, err := LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != SchemaVersion {
		t.Errorf("Version = %d, want %d", cfg.Version, SchemaVersion)
	}
	if _, err := os.Stat(ConfigFile()); !os.IsNotExist(err) {
		t.Errorf("LoadOrCreate wrote a config file %s; it should not", ConfigFile())
	}

	// And it lives at the expected place once saved.
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".skills", "config.json")); err != nil {
		t.Errorf("Save did not create config: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
