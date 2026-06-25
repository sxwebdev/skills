package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/gitutil"
	"github.com/sxwebdev/skills/internal/source"
	"github.com/urfave/cli/v3"
)

func TestNormalizeFolder(t *testing.T) {
	t.Parallel()
	// normalizeFolder slash-converts only on Windows (filepath.ToSlash is a no-op
	// on Unix) and trims a trailing separator. PathInRepo is always built with
	// OS-native separators, so on this platform the meaningful cases are the
	// already-slash forms; backslash inputs do not occur here.
	cases := []struct {
		name, in, want string
	}{
		{"plain", "skills/foo", "skills/foo"},
		{"trailing slash", "skills/foo/", "skills/foo"},
		{"nested", "a/b/c", "a/b/c"},
		{"empty", "", ""},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeFolder(tt.in); got != tt.want {
				t.Errorf("normalizeFolder(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFolderChanged(t *testing.T) {
	t.Parallel()

	// A real folder whose on-disk SHA1 we can compute, to exercise the
	// mixed-kind recompute branch the way update does on a legacy baseline.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	sha1Hash, err := gitutil.ComputeFolderHash(dir)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		skill   config.SkillInfo
		newHash string
		newKind string
		want    bool
	}{
		{
			name:    "same kind equal hash -> unchanged",
			skill:   config.SkillInfo{FolderHash: "abc", HashKind: config.HashKindTreeSHA},
			newHash: "abc", newKind: config.HashKindTreeSHA, want: false,
		},
		{
			name:    "same kind different hash -> changed",
			skill:   config.SkillInfo{FolderHash: "abc", HashKind: config.HashKindTreeSHA},
			newHash: "xyz", newKind: config.HashKindTreeSHA, want: true,
		},
		{
			name:    "legacy sha1 baseline still matches on-disk -> unchanged",
			skill:   config.SkillInfo{FolderHash: sha1Hash, HashKind: config.HashKindSHA1},
			newHash: "treesha", newKind: config.HashKindTreeSHA, want: false,
		},
		{
			name:    "legacy sha1 baseline differs from on-disk -> changed",
			skill:   config.SkillInfo{FolderHash: "deadbeef", HashKind: config.HashKindSHA1},
			newHash: "treesha", newKind: config.HashKindTreeSHA, want: true,
		},
		{
			name:    "empty stored kind treated as sha1 -> unchanged when on-disk matches",
			skill:   config.SkillInfo{FolderHash: sha1Hash, HashKind: ""},
			newHash: "treesha", newKind: config.HashKindTreeSHA, want: false,
		},
		{
			name:    "unknown stored kind cannot be recomputed -> changed",
			skill:   config.SkillInfo{FolderHash: "abc", HashKind: "md5"},
			newHash: "treesha", newKind: config.HashKindTreeSHA, want: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := folderChanged(tt.skill, dir, tt.newHash, tt.newKind)
			if err != nil {
				t.Fatalf("folderChanged: %v", err)
			}
			if got != tt.want {
				t.Errorf("folderChanged = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFolderChangedRecomputeError(t *testing.T) {
	t.Parallel()
	// A non-existent dir makes ComputeFolderHash fail, so the recompute branch
	// must surface the error rather than report a (false) change.
	skill := config.SkillInfo{FolderHash: "x", HashKind: config.HashKindSHA1}
	_, err := folderChanged(skill, filepath.Join(t.TempDir(), "missing"), "y", config.HashKindTreeSHA)
	if err == nil {
		t.Fatal("expected error from recompute on missing dir, got nil")
	}
}

func TestDiscoverNewSkillsInstallsSelected(t *testing.T) {
	repo := writeSkillRepo(t, map[string]string{"alpha": "A", "beta": "B"})
	src, f, repoKey := localSource(t, repo)
	cfg := newConfig()
	projectRoot := t.TempDir()

	// Non-interactive: prompt.SelectSkills returns every candidate, so all new
	// skills are installed and recorded.
	var n int
	cmdWith(t, []cli.Flag{agentFlag()}, nil, func(c *cli.Command) {
		n = discoverNewSkills(c, cfg, func() *source.Fetched { return &f }, src, repoKey, projectRoot, false)
	})

	if n != 2 {
		t.Fatalf("installed = %d, want 2", n)
	}
	for _, name := range []string{"alpha", "beta"} {
		s, ok := cfg.Skills[name]
		if !ok {
			t.Fatalf("skill %q not recorded", name)
		}
		if s.Repo != repoKey {
			t.Errorf("%s.Repo = %q, want %q", name, s.Repo, repoKey)
		}
		if s.PathInRepo != filepath.Join("skills", name) {
			t.Errorf("%s.PathInRepo = %q", name, s.PathInRepo)
		}
		if s.FolderHash == "" || s.HashKind != config.HashKindSHA1 {
			t.Errorf("%s hash = %q/%q, want non-empty sha1", name, s.FolderHash, s.HashKind)
		}
		if s.Project != projectRoot {
			t.Errorf("%s.Project = %q, want %q", name, s.Project, projectRoot)
		}
		if _, err := os.Stat(filepath.Join(projectRoot, ".agents", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("%s not installed on disk: %v", name, err)
		}
	}
	if _, ok := cfg.Repos[repoKey]; !ok {
		t.Error("repo not registered after installing new skills")
	}
}

func TestDiscoverNewSkillsSkipsAlreadyInstalled(t *testing.T) {
	repo := writeSkillRepo(t, map[string]string{"alpha": "A", "beta": "B"})
	src, f, repoKey := localSource(t, repo)
	cfg := newConfig()
	cfg.Skills["alpha"] = config.SkillInfo{Repo: repoKey, PathInRepo: "skills/alpha"}
	projectRoot := t.TempDir()

	var n int
	cmdWith(t, []cli.Flag{agentFlag()}, nil, func(c *cli.Command) {
		n = discoverNewSkills(c, cfg, func() *source.Fetched { return &f }, src, repoKey, projectRoot, false)
	})

	if n != 1 {
		t.Fatalf("installed = %d, want 1 (only beta is new)", n)
	}
	if _, ok := cfg.Skills["beta"]; !ok {
		t.Error("beta should have been installed")
	}
	// alpha was a pre-existing stub; it must not have been re-installed on disk.
	if _, err := os.Stat(filepath.Join(projectRoot, ".agents", "skills", "alpha")); !os.IsNotExist(err) {
		t.Error("alpha was unexpectedly (re)installed")
	}
}

func TestDiscoverNewSkillsDryRunInstallsNothing(t *testing.T) {
	repo := writeSkillRepo(t, map[string]string{"alpha": "A", "beta": "B"})
	src, f, repoKey := localSource(t, repo)
	cfg := newConfig()
	projectRoot := t.TempDir()

	var n int
	cmdWith(t, []cli.Flag{agentFlag()}, nil, func(c *cli.Command) {
		n = discoverNewSkills(c, cfg, func() *source.Fetched { return &f }, src, repoKey, projectRoot, true)
	})

	if n != 2 {
		t.Fatalf("would-install count = %d, want 2", n)
	}
	if len(cfg.Skills) != 0 {
		t.Errorf("dry-run mutated config: %v", cfg.Skills)
	}
	if entries, _ := os.ReadDir(projectRoot); len(entries) != 0 {
		t.Errorf("dry-run wrote to disk: %v", entries)
	}
}

func TestDiscoverNewSkillsNoneNew(t *testing.T) {
	repo := writeSkillRepo(t, map[string]string{"alpha": "A"})
	src, f, repoKey := localSource(t, repo)
	cfg := newConfig()
	cfg.Skills["alpha"] = config.SkillInfo{Repo: repoKey, PathInRepo: "skills/alpha"}

	var n int
	cmdWith(t, []cli.Flag{agentFlag()}, nil, func(c *cli.Command) {
		n = discoverNewSkills(c, cfg, func() *source.Fetched { return &f }, src, repoKey, t.TempDir(), false)
	})
	if n != 0 {
		t.Errorf("installed = %d, want 0", n)
	}
}

func TestDiscoverNewSkillsFetchFailed(t *testing.T) {
	cfg := newConfig()
	var n int
	cmdWith(t, []cli.Flag{agentFlag()}, nil, func(c *cli.Command) {
		n = discoverNewSkills(c, cfg, func() *source.Fetched { return nil }, source.Source{}, "k", t.TempDir(), false)
	})
	if n != 0 {
		t.Errorf("installed = %d, want 0 on fetch failure", n)
	}
	if len(cfg.Skills) != 0 {
		t.Error("no skills should be recorded when fetch failed")
	}
}

func TestOfferRemoveMissing(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := offerRemoveMissing(newConfig(), nil, false); got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("dry-run reports but keeps", func(t *testing.T) {
		cfg := newConfig()
		cfg.Skills["gone"] = config.SkillInfo{Repo: "r"}
		got := offerRemoveMissing(cfg, []string{"gone"}, true)
		if got != 1 {
			t.Errorf("would-remove count = %d, want 1", got)
		}
		if _, ok := cfg.Skills["gone"]; !ok {
			t.Error("dry-run must not delete the skill")
		}
	})

	t.Run("non-interactive declines removal", func(t *testing.T) {
		// In `go test` stdin is not a TTY, so prompt.Confirm returns its default
		// (false) and nothing is removed — a non-interactive update never deletes.
		cfg := newConfig()
		cfg.Skills["gone"] = config.SkillInfo{Repo: "r"}
		got := offerRemoveMissing(cfg, []string{"gone"}, false)
		if got != 0 {
			t.Errorf("removed = %d, want 0 when confirmation declined", got)
		}
		if _, ok := cfg.Skills["gone"]; !ok {
			t.Error("declined removal must keep the skill")
		}
	})

	t.Run("name absent from config is skipped", func(t *testing.T) {
		cfg := newConfig()
		if got := offerRemoveMissing(cfg, []string{"never-installed"}, false); got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})
}
