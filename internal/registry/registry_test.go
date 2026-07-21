package registry

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeSkill(t *testing.T, dir, frontmatter string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(frontmatter), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanRepo(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()

	writeSkill(t, filepath.Join(repo, "skills", "alpha"), "---\nname: alpha\ndescription: First\n---\nbody")
	writeSkill(t, filepath.Join(repo, ".agents", "skills", "beta"), "---\nname: beta\ndescription: Second\n---\nbody")
	// A skill under an agent-specific container (e.g. .claude/skills/) must also
	// be discovered, matching the documented layout and the GitHub fast-path.
	writeSkill(t, filepath.Join(repo, ".claude", "skills", "gamma"), "---\nname: gamma\ndescription: Third\n---\nbody")
	// A skill nested under an arbitrary folder must also be discovered, matching
	// layouts like questdb/skills (questdb/<skill>/SKILL.md).
	writeSkill(t, filepath.Join(repo, "questdb", "delta"), "---\nname: delta\ndescription: Fourth\n---\nbody")
	// A directory without SKILL.md should be ignored.
	if err := os.MkdirAll(filepath.Join(repo, "skills", "not-a-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A SKILL.md inside a skill's own subdirectory is a resource, not a nested
	// skill: discovery must stop at the first SKILL.md and not descend.
	writeSkill(t, filepath.Join(repo, "skills", "alpha", "reference"), "---\nname: nested\ndescription: Should be ignored\n---\nbody")

	skills, err := ScanRepo(repo)
	if err != nil {
		t.Fatal(err)
	}

	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"alpha", "beta", "delta", "gamma"}) {
		t.Errorf("ScanRepo names = %v, want [alpha beta delta gamma]", names)
	}
}

func TestReadSkillMeta(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "gamma")
	writeSkill(t, dir, "---\nname: gamma\ndescription: Third skill\n---\nbody")

	name, desc, err := ReadSkillMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	if name != "gamma" || desc != "Third skill" {
		t.Errorf("ReadSkillMeta = (%q, %q), want (gamma, Third skill)", name, desc)
	}
}

func TestReadSkillMetaFallbackName(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "delta")
	writeSkill(t, dir, "no frontmatter here")

	name, _, err := ReadSkillMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	if name != "delta" {
		t.Errorf("fallback name = %q, want delta", name)
	}
}
