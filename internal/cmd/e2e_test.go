package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sxwebdev/skills/internal/config"
)

func TestE2EAddUnknownAgentErrors(t *testing.T) {
	isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"x": "X"})
	err := AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all", "--agent", "not-an-agent"})
	if err == nil {
		t.Fatal("expected error for unknown --agent")
	}
}

func TestE2EAddDuplicateNameFromAnotherRepoIsSkipped(t *testing.T) {
	isolateHome(t)
	repoA := writeSkillRepo(t, map[string]string{"dup": "from A"})
	repoB := writeSkillRepo(t, map[string]string{"dup": "from B"})

	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repoA, "--global", "--all", "--agent", "claude-code"})
	})
	// Same skill name from a different source must be skipped, not overwritten.
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repoB, "--global", "--all", "--agent", "claude-code"})
	})

	got := loadConfig(t).Skills["dup"]
	if got.Description != "from A" {
		t.Errorf("dup.Description = %q, want from A (first source must win, not be overwritten)", got.Description)
	}
}

func TestE2EUpdateDryRun(t *testing.T) {
	isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"d": "D"})
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all", "--agent", "claude-code"})
	})
	before := loadConfig(t).Skills["d"].FolderHash

	writeSkill(t, repo, "d", "D edited")
	out := captureStdout(t, func() {
		mustRun(t, func() error {
			return UpdateCmd().Run(t.Context(), []string{"update", "--global", "--dry-run"})
		})
	})
	if !strings.Contains(out, "would update") {
		t.Errorf("dry-run did not report a pending update:\n%s", out)
	}
	if after := loadConfig(t).Skills["d"].FolderHash; after != before {
		t.Error("dry-run must not change the recorded hash")
	}
}

func TestE2EUpdateResolvesAgentsForLegacyEntry(t *testing.T) {
	isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"d": "D"})
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all", "--agent", "claude-code"})
	})

	// Simulate a legacy entry with no recorded agents, forcing update to
	// re-resolve them from flags/config.
	cfg := loadConfig(t)
	s := cfg.Skills["d"]
	s.Agents = nil
	cfg.Skills["d"] = s
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	writeSkill(t, repo, "d", "D edited")
	mustRun(t, func() error {
		return UpdateCmd().Run(t.Context(), []string{"update", "--global", "--agent", "claude-code"})
	})
	if got := loadConfig(t).Skills["d"].Agents; len(got) != 1 || got[0] != "claude-code" {
		t.Errorf("agents after update = %v, want [claude-code]", got)
	}
}

func TestE2ERemoveSourceWithoutYesConfirmsByDefault(t *testing.T) {
	isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"one": "1"})
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all", "--agent", "claude-code"})
	})
	// removeSource's confirm defaults to true, so a non-interactive run proceeds.
	mustRun(t, func() error {
		return RemoveCmd().Run(t.Context(), []string{"remove", repo})
	})
	if len(loadConfig(t).Skills) != 0 {
		t.Error("source removal should have proceeded by default")
	}
}

// isolateHome points config + global install dirs at a fresh temp HOME so the
// real user's ~/.skills is never touched. t.Setenv forbids t.Parallel, which is
// fine: these end-to-end cases own process-global state (HOME, stdout).
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func loadConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func TestE2EAddUpdateRemove(t *testing.T) {
	home := isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"one": "first", "two": "second"})

	// add --all installs every skill in the source globally.
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all", "--agent", "claude-code"})
	})

	cfg := loadConfig(t)
	if len(cfg.Skills) != 2 {
		t.Fatalf("after add: %d skills, want 2", len(cfg.Skills))
	}
	for _, name := range []string{"one", "two"} {
		if _, err := os.Stat(filepath.Join(home, ".agents", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("%s not installed: %v", name, err)
		}
	}
	firstHash := cfg.Skills["one"].FolderHash

	// A second update with nothing changed must report no updates and keep hashes.
	mustRun(t, func() error {
		return UpdateCmd().Run(t.Context(), []string{"update", "--global"})
	})
	if got := loadConfig(t).Skills["one"].FolderHash; got != firstHash {
		t.Errorf("unchanged skill hash moved: %q -> %q", firstHash, got)
	}

	// Change a skill in the source; update must re-install it and advance the hash.
	writeSkill(t, repo, "one", "first, edited")
	mustRun(t, func() error {
		return UpdateCmd().Run(t.Context(), []string{"update", "--global"})
	})
	updated := loadConfig(t).Skills["one"]
	if updated.FolderHash == firstHash {
		t.Error("changed skill hash did not advance after update")
	}
	if !updated.UpdatedAt.After(updated.InstalledAt) && updated.UpdatedAt.Equal(updated.InstalledAt) {
		// UpdatedAt should be >= InstalledAt; just ensure it is set.
		t.Error("UpdatedAt not refreshed")
	}

	// list and doctor walk the installed set; they must succeed on a healthy config.
	mustRun(t, func() error { return ListCmd().Run(t.Context(), []string{"list"}) })
	mustRun(t, func() error { return DoctorCmd().Run(t.Context(), []string{"doctor"}) })

	// remove --all clears everything and prunes the now-empty source.
	mustRun(t, func() error {
		return RemoveCmd().Run(t.Context(), []string{"remove", "--all", "--global", "--yes"})
	})
	cfg = loadConfig(t)
	if len(cfg.Skills) != 0 {
		t.Errorf("after remove: %d skills, want 0", len(cfg.Skills))
	}
	if len(cfg.Repos) != 0 {
		t.Errorf("after remove: repo not pruned: %v", cfg.Repos)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "one")); !os.IsNotExist(err) {
		t.Error("install dir lingered after remove")
	}
}

func TestE2EAddList(t *testing.T) {
	isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"a": "A", "b": "B"})

	// --list prints the catalog without installing anything.
	out := captureStdout(t, func() {
		mustRun(t, func() error {
			return AddCmd().Run(t.Context(), []string{"add", repo, "--list"})
		})
	})
	if cfg, err := config.Load(); err == nil && len(cfg.Skills) != 0 {
		t.Errorf("--list must not install: %v", cfg.Skills)
	}
	for _, name := range []string{"a", "b"} {
		if !strings.Contains(out, name) {
			t.Errorf("--list output missing %q:\n%s", name, out)
		}
	}
}

func TestE2EAddSingleViaSelector(t *testing.T) {
	home := isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"keep": "K", "skip": "S"})

	// @skill narrows the install to one skill without an interactive prompt.
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repo + "@keep", "--global", "--agent", "claude-code"})
	})
	cfg := loadConfig(t)
	if _, ok := cfg.Skills["keep"]; !ok {
		t.Error("selected skill not installed")
	}
	if _, ok := cfg.Skills["skip"]; ok {
		t.Error("non-selected skill should not be installed")
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "skip")); !os.IsNotExist(err) {
		t.Error("skip should not be on disk")
	}
}

func TestE2ERemoveSource(t *testing.T) {
	home := isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"one": "1", "two": "2"})
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all", "--agent", "claude-code"})
	})

	// `remove <source>` matches a registered source and removes all its skills,
	// then unregisters it (removeSource path).
	mustRun(t, func() error {
		return RemoveCmd().Run(t.Context(), []string{"remove", repo, "--yes"})
	})
	cfg := loadConfig(t)
	if len(cfg.Skills) != 0 || len(cfg.Repos) != 0 {
		t.Errorf("source removal left state: skills=%v repos=%v", cfg.Skills, cfg.Repos)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "one")); !os.IsNotExist(err) {
		t.Error("skills not removed from disk")
	}
}

func TestE2ERemoveSourceKeepSkills(t *testing.T) {
	isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"one": "1"})
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all", "--agent", "claude-code"})
	})

	// --keep-skills unregisters the source but leaves the installed skills.
	mustRun(t, func() error {
		return RemoveCmd().Run(t.Context(), []string{"remove", repo, "--keep-skills", "--yes"})
	})
	cfg := loadConfig(t)
	if len(cfg.Repos) != 0 {
		t.Error("source should be unregistered")
	}
	if _, ok := cfg.Skills["one"]; !ok {
		t.Error("--keep-skills should leave the skill installed")
	}
}

func TestE2EUpdateNoSkills(t *testing.T) {
	isolateHome(t)
	// Fresh config, nothing installed: update is a no-op, not an error.
	mustRun(t, func() error {
		return UpdateCmd().Run(t.Context(), []string{"update", "--global"})
	})
}

func TestE2EAddUnknownSourceErrors(t *testing.T) {
	isolateHome(t)
	err := AddCmd().Run(t.Context(), []string{"add", filepath.Join(t.TempDir(), "does-not-exist"), "--global"})
	if err == nil {
		t.Fatal("expected error adding a non-existent local source")
	}
}

func TestE2EAddEmptyAndFilterMiss(t *testing.T) {
	isolateHome(t)

	// A source with no skills is reported, not an error.
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", t.TempDir(), "--global"})
	})

	repo := writeSkillRepo(t, map[string]string{"real": "R"})
	// --skill that matches nothing is an error.
	err := AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--skill", "ghost"})
	if err == nil {
		t.Fatal("expected error for non-matching --skill filter")
	}
}

func TestE2EAddCopyMode(t *testing.T) {
	isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"c": "C"})
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all", "--copy", "--agent", "claude-code"})
	})
	if got := loadConfig(t).Skills["c"].Mode; got != config.ModeCopy {
		t.Errorf("mode = %q, want copy", got)
	}
}

func TestE2EUpdateVanishedSkillNonInteractive(t *testing.T) {
	isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"keep": "K", "vanish": "V"})
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all", "--agent", "claude-code"})
	})

	// Delete a skill from the source. A non-interactive update must warn but, per
	// the safety rule, never auto-remove it.
	if err := os.RemoveAll(filepath.Join(repo, "skills", "vanish")); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		mustRun(t, func() error {
			return UpdateCmd().Run(t.Context(), []string{"update", "--global"})
		})
	})
	if !strings.Contains(out, "no longer exists in source") {
		t.Errorf("update did not warn about the vanished skill:\n%s", out)
	}
	if _, ok := loadConfig(t).Skills["vanish"]; !ok {
		t.Error("non-interactive update must not delete the vanished skill")
	}
}

func TestE2EUpdateNameFiltered(t *testing.T) {
	isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"a": "A", "b": "B"})
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all", "--agent", "claude-code"})
	})
	before := loadConfig(t)
	hashA, hashB := before.Skills["a"].FolderHash, before.Skills["b"].FolderHash

	// Change both, but update only "a": b must be untouched.
	writeSkill(t, repo, "a", "A edited")
	writeSkill(t, repo, "b", "B edited")
	mustRun(t, func() error {
		return UpdateCmd().Run(t.Context(), []string{"update", "a", "--global"})
	})
	after := loadConfig(t)
	if after.Skills["a"].FolderHash == hashA {
		t.Error("filtered skill 'a' was not updated")
	}
	if after.Skills["b"].FolderHash != hashB {
		t.Error("unfiltered skill 'b' should not have been updated")
	}
}

func TestE2ERemoveVariants(t *testing.T) {
	isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"one": "1", "two": "2"})
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all", "--agent", "claude-code"})
	})

	// Remove a single named skill.
	mustRun(t, func() error {
		return RemoveCmd().Run(t.Context(), []string{"remove", "one", "--global", "--yes"})
	})
	cfg := loadConfig(t)
	if _, ok := cfg.Skills["one"]; ok {
		t.Error("named skill not removed")
	}
	if _, ok := cfg.Skills["two"]; !ok {
		t.Error("sibling skill should remain")
	}

	// Removing a name that is not installed warns but does not error.
	mustRun(t, func() error {
		return RemoveCmd().Run(t.Context(), []string{"remove", "ghost", "--global", "--yes"})
	})

	// No args is a usage error.
	if err := RemoveCmd().Run(t.Context(), []string{"remove"}); err == nil {
		t.Error("expected usage error for `remove` with no args")
	}
}

func TestE2ERemoveDeclinedNonInteractive(t *testing.T) {
	isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"keep": "K"})
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all", "--agent", "claude-code"})
	})

	// Without --yes and with no TTY, prompt.Confirm returns its default (false),
	// so removal is cancelled and the skill stays.
	mustRun(t, func() error {
		return RemoveCmd().Run(t.Context(), []string{"remove", "keep", "--global"})
	})
	if _, ok := loadConfig(t).Skills["keep"]; !ok {
		t.Error("declined removal must keep the skill")
	}
}

func TestE2EFindAndListJSON(t *testing.T) {
	isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"x": "X"})
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all", "--agent", "claude-code"})
	})
	out := captureStdout(t, func() {
		mustRun(t, func() error { return ListCmd().Run(t.Context(), []string{"list", "--json"}) })
	})
	if !strings.Contains(out, "\"x\"") {
		t.Errorf("list --json missing skill:\n%s", out)
	}
}

func TestE2EDoctorReportsBrokenSkill(t *testing.T) {
	home := isolateHome(t)
	repo := writeSkillRepo(t, map[string]string{"broken": "B"})
	mustRun(t, func() error {
		return AddCmd().Run(t.Context(), []string{"add", repo, "--global", "--all", "--agent", "claude-code"})
	})
	// Break the install so doctor has an issue to report.
	if err := os.RemoveAll(filepath.Join(home, ".agents", "skills", "broken")); err != nil {
		t.Fatal(err)
	}
	// doctor reports issues via ui.Error, which writes to stderr.
	out := captureStderr(t, func() {
		mustRun(t, func() error { return DoctorCmd().Run(t.Context(), []string{"doctor"}) })
	})
	if !strings.Contains(out, "broken") {
		t.Errorf("doctor did not mention the broken skill:\n%s", out)
	}
}

// mustRun fails the test if the command returns an error.
func mustRun(t *testing.T, fn func() error) {
	t.Helper()
	if err := fn(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
}

func containsName(haystack, name string) bool {
	for i := 0; i+len(name) <= len(haystack); i++ {
		if haystack[i:i+len(name)] == name {
			return true
		}
	}
	return false
}
