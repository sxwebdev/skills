package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSrcSkill(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: s\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return src
}

func TestInstallSkillSymlink(t *testing.T) {
	root := t.TempDir()
	src := writeSrcSkill(t)

	mode, err := InstallSkill(InstallOpts{
		Name: "s", SrcDir: src, Agents: []string{"claude-code"}, ProjectRoot: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if mode != "symlink" {
		t.Errorf("mode = %q, want symlink", mode)
	}

	link := filepath.Join(root, ".claude", "skills", "s")
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("%s is not a symlink", link)
	}
	if _, err := os.Stat(filepath.Join(link, "SKILL.md")); err != nil {
		t.Errorf("symlink does not resolve to SKILL.md: %v", err)
	}
}

func TestInstallSkillForceCopy(t *testing.T) {
	root := t.TempDir()
	src := writeSrcSkill(t)

	mode, err := InstallSkill(InstallOpts{
		Name: "s", SrcDir: src, Agents: []string{"claude-code"}, ProjectRoot: root, ForceCopy: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if mode != "copy" {
		t.Errorf("mode = %q, want copy", mode)
	}

	link := filepath.Join(root, ".claude", "skills", "s")
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Errorf("%s should be a real directory, not a symlink", link)
	}
	if _, err := os.Stat(filepath.Join(link, "SKILL.md")); err != nil {
		t.Errorf("copied dir missing SKILL.md: %v", err)
	}
}

func TestRemoveSkillCleansBothModes(t *testing.T) {
	for _, forceCopy := range []bool{false, true} {
		root := t.TempDir()
		src := writeSrcSkill(t)
		if _, err := InstallSkill(InstallOpts{
			Name: "s", SrcDir: src, Agents: []string{"claude-code"}, ProjectRoot: root, ForceCopy: forceCopy,
		}); err != nil {
			t.Fatal(err)
		}

		if err := RemoveSkill("s", []string{"claude-code"}, root); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(filepath.Join(root, ".claude", "skills", "s")); !os.IsNotExist(err) {
			t.Errorf("forceCopy=%v: agent link not removed", forceCopy)
		}
		if _, err := os.Lstat(filepath.Join(root, ".agents", "skills", "s")); !os.IsNotExist(err) {
			t.Errorf("forceCopy=%v: skill dir not removed", forceCopy)
		}
	}
}

func TestInstallSkillRejectsUnsafeName(t *testing.T) {
	root := t.TempDir()
	src := writeSrcSkill(t)
	if _, err := InstallSkill(InstallOpts{Name: "../evil", SrcDir: src, ProjectRoot: root}); err == nil {
		t.Fatal("expected error for unsafe skill name")
	}
}
