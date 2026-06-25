package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sxwebdev/skills/internal/config"
)

func TestInScope(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		project     string
		projectRoot string
		want        bool
	}{
		{"global scope matches everything", "/p", "", true},
		{"global scope matches global skill", "", "", true},
		{"project scope matches same project", "/p", "/p", true},
		{"project scope rejects other project", "/q", "/p", false},
		{"project scope rejects global skill", "", "/p", false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := inScope(config.SkillInfo{Project: tt.project}, tt.projectRoot)
			if got != tt.want {
				t.Errorf("inScope(project=%q, root=%q) = %v, want %v", tt.project, tt.projectRoot, got, tt.want)
			}
		})
	}
}

func TestPruneRepo(t *testing.T) {
	t.Parallel()

	t.Run("keeps repo with remaining skills", func(t *testing.T) {
		t.Parallel()
		cfg := newConfig()
		cfg.Repos["r"] = config.RepoInfo{Alias: "r"}
		cfg.Skills["keep"] = config.SkillInfo{Repo: "r"}
		pruneRepo(cfg, "r")
		if _, ok := cfg.Repos["r"]; !ok {
			t.Error("repo with a referencing skill was pruned")
		}
	})

	t.Run("drops repo once unreferenced", func(t *testing.T) {
		t.Parallel()
		cfg := newConfig()
		cfg.Repos["r"] = config.RepoInfo{Alias: "r"}
		pruneRepo(cfg, "r")
		if _, ok := cfg.Repos["r"]; ok {
			t.Error("unreferenced repo was not pruned")
		}
	})
}

func TestRemoveOne(t *testing.T) {
	t.Run("uninstalls and prunes last skill of a repo", func(t *testing.T) {
		root := t.TempDir()
		skill, repoKey := installFixture(t, root, "solo")
		cfg := newConfig()
		cfg.Repos[repoKey] = config.RepoInfo{Alias: "r"}
		cfg.Skills["solo"] = skill

		if err := removeOne(cfg, "solo", skill); err != nil {
			t.Fatalf("removeOne: %v", err)
		}
		if _, ok := cfg.Skills["solo"]; ok {
			t.Error("skill not deleted from config")
		}
		if _, ok := cfg.Repos[repoKey]; ok {
			t.Error("repo not pruned after last skill removed")
		}
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "solo")); !os.IsNotExist(err) {
			t.Error("install dir not removed")
		}
		if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "solo")); !os.IsNotExist(err) {
			t.Error("agent link not removed")
		}
	})

	t.Run("keeps repo with a sibling skill", func(t *testing.T) {
		root := t.TempDir()
		skill, repoKey := installFixture(t, root, "one")
		cfg := newConfig()
		cfg.Repos[repoKey] = config.RepoInfo{Alias: "r"}
		cfg.Skills["one"] = skill
		cfg.Skills["two"] = config.SkillInfo{Repo: repoKey} // sibling keeps the repo alive

		if err := removeOne(cfg, "one", skill); err != nil {
			t.Fatalf("removeOne: %v", err)
		}
		if _, ok := cfg.Repos[repoKey]; !ok {
			t.Error("repo pruned despite a remaining sibling skill")
		}
	})
}

func TestMatchSource(t *testing.T) {
	t.Parallel()

	t.Run("direct key hit", func(t *testing.T) {
		t.Parallel()
		cfg := newConfig()
		cfg.Repos["https://github.com/o/r.git"] = config.RepoInfo{Alias: "o/r"}
		got, ok := matchSource(cfg, "https://github.com/o/r.git")
		if !ok || got != "https://github.com/o/r.git" {
			t.Errorf("matchSource direct = %q,%v", got, ok)
		}
	})

	t.Run("resolves via parsed clone URL", func(t *testing.T) {
		t.Parallel()
		cfg := newConfig()
		cfg.Repos["https://github.com/o/r.git"] = config.RepoInfo{Alias: "o/r"}
		// "o/r" shorthand parses to the same github clone URL without any network.
		got, ok := matchSource(cfg, "o/r")
		if !ok || got != "https://github.com/o/r.git" {
			t.Errorf("matchSource shorthand = %q,%v", got, ok)
		}
	})

	t.Run("resolves via alias", func(t *testing.T) {
		t.Parallel()
		cfg := newConfig()
		// Key differs from the parsed URL, but the alias matches the parsed Alias.
		cfg.Repos["some-other-key"] = config.RepoInfo{Alias: "o/r"}
		got, ok := matchSource(cfg, "o/r")
		if !ok || got != "some-other-key" {
			t.Errorf("matchSource alias = %q,%v", got, ok)
		}
	})

	t.Run("unregistered source does not match", func(t *testing.T) {
		t.Parallel()
		cfg := newConfig()
		if got, ok := matchSource(cfg, "o/r"); ok {
			t.Errorf("matchSource unexpectedly matched: %q", got)
		}
	})

	t.Run("unparseable arg does not match", func(t *testing.T) {
		t.Parallel()
		cfg := newConfig()
		if _, ok := matchSource(cfg, ""); ok {
			t.Error("empty arg should not match")
		}
	})
}
