package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/registry"
	"github.com/sxwebdev/skills/internal/source"
)

func TestInstallAndRecord(t *testing.T) {
	t.Run("installs, records every field, clones the agent list", func(t *testing.T) {
		repo := writeSkillRepo(t, map[string]string{"foo": "the foo skill"})
		src, f, repoKey := localSource(t, repo)
		src.Ref = "v1"
		src.Subpath = "sub"
		skill := discover(t, f, "foo")
		root := t.TempDir()

		agentList := []string{"claude-code"}
		cfg := newConfig()
		if err := installAndRecord(cfg, &f, src, repoKey, skill, agentList, root, false); err != nil {
			t.Fatalf("installAndRecord: %v", err)
		}

		// Mutating the caller's slice afterwards must not change the stored agents
		// (installAndRecord clones it).
		agentList[0] = "mutated"

		got, ok := cfg.Skills["foo"]
		if !ok {
			t.Fatal("skill not recorded")
		}
		want := config.SkillInfo{
			Repo:        repoKey,
			Description: "the foo skill",
			PathInRepo:  filepath.Join("skills", "foo"),
			HashKind:    config.HashKindSHA1,
			Agents:      []string{"claude-code"},
			Mode:        config.ModeSymlink,
			Ref:         "v1",
			Subpath:     "sub",
			Project:     root,
		}
		if got.Repo != want.Repo || got.Description != want.Description || got.PathInRepo != want.PathInRepo {
			t.Errorf("identity mismatch: %+v", got)
		}
		if got.HashKind != want.HashKind || got.FolderHash == "" {
			t.Errorf("hash = %q/%q, want non-empty %s", got.FolderHash, got.HashKind, want.HashKind)
		}
		if len(got.Agents) != 1 || got.Agents[0] != "claude-code" {
			t.Errorf("agents = %v, want [claude-code] (independent of caller slice)", got.Agents)
		}
		if got.Mode != want.Mode || got.Ref != want.Ref || got.Subpath != want.Subpath || got.Project != want.Project {
			t.Errorf("metadata mismatch: %+v", got)
		}
		if got.InstalledAt.IsZero() || got.UpdatedAt.IsZero() {
			t.Error("timestamps not set")
		}
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "foo", "SKILL.md")); err != nil {
			t.Errorf("skill not installed on disk: %v", err)
		}
	})

	t.Run("forceCopy records copy mode", func(t *testing.T) {
		repo := writeSkillRepo(t, map[string]string{"foo": "d"})
		src, f, repoKey := localSource(t, repo)
		skill := discover(t, f, "foo")
		cfg := newConfig()

		if err := installAndRecord(cfg, &f, src, repoKey, skill, []string{"claude-code"}, t.TempDir(), true); err != nil {
			t.Fatalf("installAndRecord: %v", err)
		}
		if cfg.Skills["foo"].Mode != config.ModeCopy {
			t.Errorf("mode = %q, want copy", cfg.Skills["foo"].Mode)
		}
	})

	t.Run("install failure returns error and records nothing", func(t *testing.T) {
		_, f, repoKey := localSource(t, writeSkillRepo(t, map[string]string{"foo": "d"}))
		// A skill whose AbsPath does not exist makes InstallSkill fail.
		bad := registry.DiscoveredSkill{Name: "ghost", PathInRepo: "skills/ghost", AbsPath: filepath.Join(t.TempDir(), "nope")}
		cfg := newConfig()

		err := installAndRecord(cfg, &f, source.Source{}, repoKey, bad, []string{"claude-code"}, t.TempDir(), false)
		if err == nil {
			t.Fatal("expected install error, got nil")
		}
		if _, ok := cfg.Skills["ghost"]; ok {
			t.Error("failed install must not record a skill")
		}
	})
}

func TestFilterByNames(t *testing.T) {
	t.Parallel()
	all := []registry.DiscoveredSkill{{Name: "a"}, {Name: "b"}, {Name: "c"}}

	cases := []struct {
		name  string
		names []string
		want  []string
	}{
		{"subset preserves source order", []string{"c", "a"}, []string{"a", "c"}},
		{"no overlap yields empty", []string{"x", "y"}, nil},
		{"duplicates in filter do not duplicate output", []string{"a", "a"}, []string{"a"}},
		{"empty filter yields empty", nil, nil},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := filterByNames(all, tt.names)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (%v)", len(got), len(tt.want), got)
			}
			for i, s := range got {
				if s.Name != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, s.Name, tt.want[i])
				}
			}
		})
	}
}
