package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/urfave/cli/v3"
)

// writeCorruptConfig drops a malformed config.json into the isolated HOME so the
// config-load error branch fires for every command that loads it.
func writeCorruptConfig(t *testing.T) {
	t.Helper()
	dir := config.ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigFile(), []byte("{ this is not json"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCorruptConfigSurfacesError(t *testing.T) {
	isolateHome(t)
	writeCorruptConfig(t)
	repo := writeSkillRepo(t, map[string]string{"x": "X"})

	cmds := []struct {
		name string
		run  func() error
	}{
		{"add", func() error { return AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all"}) }},
		{"update", func() error { return UpdateCmd().Run(t.Context(), []string{"update", "--global"}) }},
		{"remove", func() error { return RemoveCmd().Run(t.Context(), []string{"remove", "x", "--global", "--yes"}) }},
		{"find", func() error { return FindCmd().Run(t.Context(), []string{"find"}) }},
		{"list", func() error { return ListCmd().Run(t.Context(), []string{"list"}) }},
	}
	for _, c := range cmds {
		t.Run(c.name, func(t *testing.T) {
			if err := c.run(); err == nil {
				t.Errorf("%s should surface the corrupt-config error", c.name)
			}
		})
	}

	// doctor handles a non-ErrNotExist config error gracefully (reports, no error).
	if err := DoctorCmd().Run(t.Context(), []string{"doctor"}); err != nil {
		t.Errorf("doctor should not return an error on a corrupt config: %v", err)
	}
}

func TestUseBadSourceErrors(t *testing.T) {
	// A local path that is not a directory makes Fetch fail.
	err := UseCmd().Run(t.Context(), []string{"use", filepath.Join(t.TempDir(), "nope")})
	if err == nil {
		t.Fatal("expected fetch error for a non-existent local source")
	}
}

func TestAddNoArgsErrors(t *testing.T) {
	isolateHome(t)
	if err := AddCmd().Run(t.Context(), []string{"add"}); err == nil {
		t.Fatal("expected usage error for `add` with no source")
	}
}

func TestDoctorNoConfig(t *testing.T) {
	isolateHome(t) // fresh HOME, no config file written yet
	out := captureStdout(t, func() {
		if err := DoctorCmd().Run(t.Context(), []string{"doctor"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "No config yet") {
		t.Errorf("doctor output = %q, want no-config message", out)
	}
}

func TestDoctorReportsOrphanedDirectory(t *testing.T) {
	home := isolateHome(t)
	// A directory in the global install dir with no matching config entry is an
	// orphan doctor must report.
	stray := filepath.Join(home, ".agents", "skills", "stray")
	if err := os.MkdirAll(stray, 0o755); err != nil {
		t.Fatal(err)
	}
	// Need a (valid, empty) config so Load succeeds rather than ErrNotExist.
	cfg := config.NewDefault()
	if err := os.MkdirAll(config.ConfigDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		if err := DoctorCmd().Run(t.Context(), []string{"doctor"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Orphaned directory") || !strings.Contains(out, "stray") {
		t.Errorf("doctor did not report the orphan:\n%s", out)
	}
}

func TestE2EUpdateSourceGone(t *testing.T) {
	isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"x": "X"})
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all", "--agent", "claude-code"})
	})

	// The whole source directory disappears: a local Fetch then fails, and update
	// must warn rather than crash, leaving the recorded skill intact.
	if err := os.RemoveAll(repo); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		mustRun(t, func() error {
			return UpdateCmd().Run(t.Context(), []string{"update", "--global"})
		})
	})
	if !strings.Contains(out, "Failed to fetch") && !strings.Contains(out, "is not a directory") {
		t.Errorf("expected a fetch-failure warning, got:\n%s", out)
	}
	if _, ok := loadConfig(t).Skills["x"]; !ok {
		t.Error("skill should remain when its source is unreachable")
	}
}

func TestResolveProjectLocal(t *testing.T) {
	flags := func() []cli.Flag {
		return []cli.Flag{globalFlag(), &cli.StringFlag{Name: "project"}, &cli.BoolFlag{Name: "local"}}
	}

	t.Run("--local finds the nearest git root", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		sub := filepath.Join(root, "a", "b")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(sub)
		cmdWith(t, flags(), []string{"--local"}, func(c *cli.Command) {
			got, err := resolveScope(c)
			if err != nil {
				t.Fatal(err)
			}
			// macOS /var is a symlink to /private/var, so compare resolved paths.
			gotEval, _ := filepath.EvalSymlinks(got)
			wantEval, _ := filepath.EvalSymlinks(root)
			if gotEval != wantEval {
				t.Errorf("got %q, want %q", gotEval, wantEval)
			}
		})
	})

	t.Run("--local without a git root errors", func(t *testing.T) {
		t.Chdir(t.TempDir()) // no .git anywhere above a fresh temp dir... unless TMPDIR is in a repo
		cmdWith(t, flags(), []string{"--local"}, func(c *cli.Command) {
			_, err := resolveScope(c)
			if err == nil {
				t.Skip("temp dir resolved inside a git repo; cannot exercise the error path here")
			}
		})
	})
}
