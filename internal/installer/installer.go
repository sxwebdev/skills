package installer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sxwebdev/skills/internal/config"
)

// InstallSkill copies a skill from srcDir to ~/.agents/skills/<name>/
// and creates symlinks for each configured agent.
func InstallSkill(name, srcDir string, agents []string) error {
	dstDir := filepath.Join(config.SkillsInstallDir(), name)

	// Remove existing if present
	if err := os.RemoveAll(dstDir); err != nil {
		return fmt.Errorf("remove existing skill dir: %w", err)
	}

	if err := CopyDir(srcDir, dstDir); err != nil {
		return fmt.Errorf("copy skill: %w", err)
	}

	for _, agent := range agents {
		agentDir := config.AgentSkillsDir(agent)
		if agentDir == "" {
			continue
		}
		if err := os.MkdirAll(agentDir, 0o755); err != nil {
			return fmt.Errorf("create agent dir %s: %w", agentDir, err)
		}

		linkPath := filepath.Join(agentDir, name)
		// Remove existing symlink if present
		if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove existing symlink %s: %w", linkPath, err)
		}

		// Create relative symlink
		relTarget, err := filepath.Rel(agentDir, dstDir)
		if err != nil {
			return fmt.Errorf("compute relative path: %w", err)
		}
		if err := os.Symlink(relTarget, linkPath); err != nil {
			return fmt.Errorf("create symlink %s: %w", linkPath, err)
		}
	}

	return nil
}

// RemoveSkill removes a skill directory and all agent symlinks.
func RemoveSkill(name string, agents []string) error {
	// Remove agent symlinks
	for _, agent := range agents {
		agentDir := config.AgentSkillsDir(agent)
		if agentDir == "" {
			continue
		}
		linkPath := filepath.Join(agentDir, name)
		if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove symlink %s: %w", linkPath, err)
		}
	}

	// Remove skill directory
	dstDir := filepath.Join(config.SkillsInstallDir(), name)
	if err := os.RemoveAll(dstDir); err != nil {
		return fmt.Errorf("remove skill dir: %w", err)
	}

	return nil
}

// CopyDir recursively copies src directory to dst.
func CopyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .git
		if d.Name() == ".git" && d.IsDir() {
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		return os.WriteFile(target, data, info.Mode())
	})
}
