package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/installer"
	"github.com/sxwebdev/skills/internal/registry"
	"github.com/sxwebdev/skills/internal/source"
	"github.com/urfave/cli/v3"
)

// These tests live in the white-box `package cmd` because the bulk of the
// command logic worth testing is unexported (installAndRecord, discoverNewSkills,
// folderChanged, pruneRepo, removeOne, …). Black-box would force them through the
// full CLI surface, which needs a real network clone and a TTY for prompts.

// writeSkillRepo builds a local source directory containing one skill folder per
// entry of skills (name -> description), each with a valid SKILL.md, and returns
// the repo root. A local source needs no network: source.Fetch returns the dir.
func writeSkillRepo(t *testing.T, skills map[string]string) string {
	t.Helper()
	repo := t.TempDir()
	for name, desc := range skills {
		writeSkill(t, repo, name, desc)
	}
	return repo
}

// writeSkill (re)writes a single skill folder under repo/skills/<name>.
func writeSkill(t *testing.T, repo, name, desc string) {
	t.Helper()
	dir := filepath.Join(repo, "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n# %s\n", name, desc, name)
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
}

// localSource parses repoDir as a local source and fetches it. The returned
// repoKey matches how add/update key the registry for local sources (src.LocalDir).
func localSource(t *testing.T, repoDir string) (source.Source, source.Fetched, string) {
	t.Helper()
	src, err := source.Parse(repoDir)
	if err != nil {
		t.Fatalf("parse local source: %v", err)
	}
	f, err := source.Find(repoDir).Fetch(t.Context(), src)
	if err != nil {
		t.Fatalf("fetch local source: %v", err)
	}
	t.Cleanup(f.Cleanup)
	return src, f, src.LocalDir
}

// discover returns the skill discovered in repoDir by the same scanner the
// commands use, so tests build the DiscoveredSkill exactly as production does.
func discover(t *testing.T, f source.Fetched, name string) registry.DiscoveredSkill {
	t.Helper()
	skills, err := registry.ScanRepo(f.Dir)
	if err != nil {
		t.Fatalf("scan repo: %v", err)
	}
	for _, s := range skills {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("skill %q not discovered in %s", name, f.Dir)
	return registry.DiscoveredSkill{}
}

// newConfig returns an empty config with a deterministic agent list, so
// resolveAgents never falls through to environment auto-detection.
func newConfig() *config.Config {
	c := config.NewDefault()
	c.Repos = map[string]config.RepoInfo{}
	c.Skills = map[string]config.SkillInfo{}
	c.Agents = []string{"claude-code"}
	return c
}

// cmdWith runs fn inside a throwaway cli.Command populated with the given flags
// and args, giving helpers that need a *cli.Command (resolveAgents, resolveScope)
// a real, flag-populated command to read from.
func cmdWith(t *testing.T, flags []cli.Flag, args []string, fn func(*cli.Command)) {
	t.Helper()
	cmd := &cli.Command{
		Name:  "test",
		Flags: flags,
		Action: func(_ context.Context, c *cli.Command) error {
			fn(c)
			return nil
		},
	}
	if err := cmd.Run(t.Context(), append([]string{"test"}, args...)); err != nil {
		t.Fatalf("cmd.Run: %v", err)
	}
}

// installFixture installs a skill from a local repo into projectRoot and returns
// the SkillInfo that the commands would record, so link/remove tests start from a
// real on-disk install rather than a hand-built struct.
func installFixture(t *testing.T, projectRoot, name string) (config.SkillInfo, string) {
	t.Helper()
	repo := writeSkillRepo(t, map[string]string{name: "desc " + name})
	_, f, repoKey := localSource(t, repo)
	skill := discover(t, f, name)

	mode, err := installer.InstallSkill(installer.InstallOpts{
		Name: name, SrcDir: skill.AbsPath, Agents: []string{"claude-code"}, ProjectRoot: projectRoot,
	})
	if err != nil {
		t.Fatalf("install fixture: %v", err)
	}
	hash, kind, err := f.FolderHash(skill.PathInRepo)
	if err != nil {
		t.Fatalf("hash fixture: %v", err)
	}
	return config.SkillInfo{
		Repo:       repoKey,
		PathInRepo: skill.PathInRepo,
		FolderHash: hash,
		HashKind:   kind,
		Agents:     []string{"claude-code"},
		Mode:       mode,
		Project:    projectRoot,
	}, repoKey
}
