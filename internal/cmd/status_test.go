package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sxwebdev/skills/internal/config"
)

func TestScopeOf(t *testing.T) {
	t.Parallel()
	if got := scopeOf(config.SkillInfo{Project: ""}); got != "global" {
		t.Errorf("global scope = %q, want global", got)
	}
	if got := scopeOf(config.SkillInfo{Project: "/x/y"}); got != "/x/y" {
		t.Errorf("project scope = %q, want /x/y", got)
	}
}

// mkSkillDir creates the install directory skillLinkIssues stats first, so the
// "directory missing" early return is not what we are exercising.
func mkSkillDir(t *testing.T, project, name string) string {
	t.Helper()
	dir := filepath.Join(config.ResolveSkillsInstallDir(project), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSkillLinkIssues(t *testing.T) {
	t.Run("healthy install has no issues", func(t *testing.T) {
		root := t.TempDir()
		skill, _ := installFixture(t, root, "ok")
		if issues := skillLinkIssues("ok", skill); len(issues) != 0 {
			t.Errorf("healthy skill reported issues: %v", issues)
		}
	})

	t.Run("missing install directory", func(t *testing.T) {
		root := t.TempDir()
		skill := config.SkillInfo{Project: root, Agents: []string{"claude-code"}, Mode: config.ModeSymlink}
		issues := skillLinkIssues("gone", skill)
		if len(issues) != 1 {
			t.Fatalf("issues = %v, want exactly one", issues)
		}
		if want := "directory missing"; !strings.Contains(issues[0], want) {
			t.Errorf("issue = %q, want it to mention %q", issues[0], want)
		}
	})

	t.Run("unknown agent", func(t *testing.T) {
		root := t.TempDir()
		mkSkillDir(t, root, "s")
		skill := config.SkillInfo{Project: root, Agents: []string{"definitely-not-an-agent"}, Mode: config.ModeSymlink}
		issues := skillLinkIssues("s", skill)
		if len(issues) != 1 || !strings.Contains(issues[0], "unknown agent") {
			t.Errorf("issues = %v, want unknown agent", issues)
		}
	})

	t.Run("symlink mode but no link", func(t *testing.T) {
		root := t.TempDir()
		mkSkillDir(t, root, "s")
		skill := config.SkillInfo{Project: root, Agents: []string{"claude-code"}, Mode: config.ModeSymlink}
		issues := skillLinkIssues("s", skill)
		if len(issues) != 1 || !strings.Contains(issues[0], "no symlink") {
			t.Errorf("issues = %v, want no symlink", issues)
		}
	})

	t.Run("symlink dangles to a missing target", func(t *testing.T) {
		root := t.TempDir()
		mkSkillDir(t, root, "s")
		agentDir := config.ResolveAgentSkillsDir(root, "claude-code")
		if err := os.MkdirAll(agentDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "does-not-exist"), filepath.Join(agentDir, "s")); err != nil {
			t.Fatal(err)
		}
		skill := config.SkillInfo{Project: root, Agents: []string{"claude-code"}, Mode: config.ModeSymlink}
		issues := skillLinkIssues("s", skill)
		if len(issues) != 1 || !strings.Contains(issues[0], "broken symlink") {
			t.Errorf("issues = %v, want broken symlink", issues)
		}
	})

	t.Run("copy mode but no copy present", func(t *testing.T) {
		root := t.TempDir()
		mkSkillDir(t, root, "s")
		skill := config.SkillInfo{Project: root, Agents: []string{"claude-code"}, Mode: config.ModeCopy}
		issues := skillLinkIssues("s", skill)
		if len(issues) != 1 || !strings.Contains(issues[0], "missing copy") {
			t.Errorf("issues = %v, want missing copy", issues)
		}
	})
}
