package installer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/sanitize"
)

// InstallOpts configures InstallSkill.
type InstallOpts struct {
	Name        string   // skill name (validated/sanitized)
	SrcDir      string   // source directory to copy from
	Agents      []string // agents to link the skill into
	ProjectRoot string   // empty = global, otherwise project root
	ForceCopy   bool     // copy instead of symlinking into agent dirs
}

// InstallSkill copies a skill from SrcDir to the skills directory and links it
// into each configured agent. It symlinks by default and falls back to copying
// when a symlink can't be created (or when ForceCopy is set). It returns the
// mode actually used ("symlink" or "copy"). If ProjectRoot is empty, installs
// globally; otherwise installs into the project.
func InstallSkill(opts InstallOpts) (string, error) {
	name, err := sanitize.Name(opts.Name)
	if err != nil {
		return "", err
	}

	installBase := config.ResolveSkillsInstallDir(opts.ProjectRoot)
	dstDir := filepath.Join(installBase, name)
	if !sanitize.IsPathSafe(installBase, dstDir) {
		return "", fmt.Errorf("unsafe skill path for %q", opts.Name)
	}

	// Remove existing if present.
	if err := os.RemoveAll(dstDir); err != nil {
		return "", fmt.Errorf("remove existing skill dir: %w", err)
	}
	if err := CopyDir(opts.SrcDir, dstDir); err != nil {
		return "", fmt.Errorf("copy skill: %w", err)
	}

	mode := config.ModeSymlink
	for _, agent := range opts.Agents {
		agentDir := config.ResolveAgentSkillsDir(opts.ProjectRoot, agent)
		if agentDir == "" {
			continue
		}
		if err := os.MkdirAll(agentDir, 0o755); err != nil {
			return "", fmt.Errorf("create agent dir %s: %w", agentDir, err)
		}

		linkPath := filepath.Join(agentDir, name)
		if !sanitize.IsPathSafe(agentDir, linkPath) {
			return "", fmt.Errorf("unsafe link path for %q", opts.Name)
		}
		used, err := linkOrCopy(agentDir, dstDir, linkPath, opts.ForceCopy)
		if err != nil {
			return "", err
		}
		// If any agent required a copy, report copy as the effective mode.
		if used == config.ModeCopy {
			mode = config.ModeCopy
		}
	}

	return mode, nil
}

// linkOrCopy links dstDir into linkPath, falling back to a copy when symlinking
// fails or when forceCopy is set. Returns the mode used.
func linkOrCopy(agentDir, dstDir, linkPath string, forceCopy bool) (string, error) {
	// Remove any existing link or directory at the target.
	if err := os.RemoveAll(linkPath); err != nil {
		return "", fmt.Errorf("remove existing %s: %w", linkPath, err)
	}

	if !forceCopy {
		relTarget, err := filepath.Rel(agentDir, dstDir)
		if err == nil {
			if err := os.Symlink(relTarget, linkPath); err == nil {
				return config.ModeSymlink, nil
			}
		}
		// Fall through to copy on any symlink failure (e.g. Windows, EPERM).
	}

	if err := CopyDir(dstDir, linkPath); err != nil {
		return "", fmt.Errorf("copy skill into %s: %w", linkPath, err)
	}
	return config.ModeCopy, nil
}

// RemoveSkill removes a skill directory and all agent links (symlinks or copies).
// If projectRoot is empty, removes from global; otherwise from the project.
func RemoveSkill(name string, agents []string, projectRoot string) error {
	// Remove agent links. Use RemoveAll so copied directories are cleaned too.
	for _, agent := range agents {
		agentDir := config.ResolveAgentSkillsDir(projectRoot, agent)
		if agentDir == "" {
			continue
		}
		linkPath := filepath.Join(agentDir, name)
		if err := os.RemoveAll(linkPath); err != nil {
			return fmt.Errorf("remove link %s: %w", linkPath, err)
		}
	}

	// Remove skill directory.
	dstDir := filepath.Join(config.ResolveSkillsInstallDir(projectRoot), name)
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
