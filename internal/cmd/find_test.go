package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sxwebdev/skills/internal/config"
)

// seedFind writes a global config with two skills and returns the HOME. "beta"
// has no stored description, with an on-disk SKILL.md, to exercise the
// read-from-disk fallback in runFind.
func seedFind(t *testing.T) string {
	t.Helper()
	home := isolateHome(t)

	betaDir := filepath.Join(config.ResolveSkillsInstallDir(""), "beta")
	if err := os.MkdirAll(betaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: beta\ndescription: beta from disk\n---\n"
	if err := os.WriteFile(filepath.Join(betaDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Skills["alpha"] = config.SkillInfo{Repo: "r", Description: "alpha stored desc"}
	cfg.Skills["beta"] = config.SkillInfo{Repo: "r"} // empty description -> disk fallback
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	return home
}

func findJSON(t *testing.T, args ...string) []foundSkill {
	t.Helper()
	out := captureStdout(t, func() {
		full := append([]string{"find", "--json"}, args...)
		if err := FindCmd().Run(t.Context(), full); err != nil {
			t.Fatalf("find: %v", err)
		}
	})
	var got []foundSkill
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode find json: %v\n%s", err, out)
	}
	return got
}

func TestRunFind(t *testing.T) {
	t.Run("no query lists all, sorted, with disk-fallback description", func(t *testing.T) {
		seedFind(t)
		got := findJSON(t)
		if len(got) != 2 {
			t.Fatalf("got %d results, want 2", len(got))
		}
		if got[0].Name != "alpha" || got[1].Name != "beta" {
			t.Errorf("not sorted by name: %+v", got)
		}
		if got[1].Description != "beta from disk" {
			t.Errorf("beta description = %q, want disk fallback", got[1].Description)
		}
		if got[0].Scope != "global" {
			t.Errorf("scope = %q, want global", got[0].Scope)
		}
	})

	t.Run("query filters by name", func(t *testing.T) {
		seedFind(t)
		got := findJSON(t, "alpha")
		if len(got) != 1 || got[0].Name != "alpha" {
			t.Errorf("got %+v, want only alpha", got)
		}
	})

	t.Run("query filters by description", func(t *testing.T) {
		seedFind(t)
		got := findJSON(t, "disk")
		if len(got) != 1 || got[0].Name != "beta" {
			t.Errorf("got %+v, want only beta (matched on description)", got)
		}
	})

	t.Run("no match yields empty json array", func(t *testing.T) {
		seedFind(t)
		if got := findJSON(t, "nonexistent-query"); len(got) != 0 {
			t.Errorf("got %+v, want empty", got)
		}
	})

	t.Run("text mode reports no matches", func(t *testing.T) {
		seedFind(t)
		out := captureStdout(t, func() {
			if err := FindCmd().Run(t.Context(), []string{"find", "nonexistent-query"}); err != nil {
				t.Fatal(err)
			}
		})
		if !strings.Contains(out, "No matching skills") {
			t.Errorf("output = %q, want no-match message", out)
		}
	})

	t.Run("text mode lists matches", func(t *testing.T) {
		seedFind(t)
		out := captureStdout(t, func() {
			if err := FindCmd().Run(t.Context(), []string{"find"}); err != nil {
				t.Fatal(err)
			}
		})
		if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
			t.Errorf("output missing skills:\n%s", out)
		}
	})
}
