package cmd

import (
	"cmp"
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/ui"
	"github.com/urfave/cli/v3"
)

func DoctorCmd() *cli.Command {
	return &cli.Command{
		Name:   "doctor",
		Usage:  "Diagnose and report issues with skills installation",
		Action: runDoctor,
	}
}

func runDoctor(_ context.Context, _ *cli.Command) error {
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			ui.Info("No config yet — nothing installed. Run 'skills add <source>'.")
			return nil
		}
		ui.Error("Config: %v", err)
		return nil
	}
	ui.Success("Config loaded (v%d)", cfg.Version)

	issues := 0

	for name, skill := range cfg.Skills {
		scope := cmp.Or(skill.Project, "global")
		skillDir := filepath.Join(config.ResolveSkillsInstallDir(skill.Project), name)

		if _, err := os.Stat(skillDir); os.IsNotExist(err) {
			ui.Error("Skill %q [%s]: directory missing at %s", name, scope, skillDir)
			issues++
			continue
		}

		if skill.Repo != "" {
			if _, exists := cfg.Repos[skill.Repo]; !exists && skill.Project == "" {
				ui.Warn("Skill %q [%s]: source %s not registered (orphaned)", name, scope, skill.Repo)
				issues++
			}
		}

		for _, agent := range skill.Agents {
			agentDir := config.ResolveAgentSkillsDir(skill.Project, agent)
			if agentDir == "" {
				ui.Warn("Skill %q: unknown agent %q", name, agent)
				issues++
				continue
			}
			linkPath := filepath.Join(agentDir, name)

			if skill.Mode == config.ModeCopy {
				if _, err := os.Stat(linkPath); err != nil {
					ui.Error("Skill %q [%s]: missing copy for %s", name, scope, agent)
					issues++
				}
				continue
			}

			target, err := os.Readlink(linkPath)
			if err != nil {
				ui.Error("Skill %q [%s]: symlink missing for %s", name, scope, agent)
				issues++
				continue
			}
			abs := target
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(agentDir, target)
			}
			if _, err := os.Stat(abs); os.IsNotExist(err) {
				ui.Error("Skill %q [%s]: broken symlink for %s → %s", name, scope, agent, target)
				issues++
			}
		}
	}

	// Orphaned directories in the global install dir.
	if entries, err := os.ReadDir(config.SkillsInstallDir()); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skill, exists := cfg.Skills[entry.Name()]
			if !exists || skill.Project != "" {
				ui.Warn("Orphaned directory: %s", filepath.Join(config.SkillsInstallDir(), entry.Name()))
				issues++
			}
		}
	}

	if issues == 0 {
		ui.Success("No issues found!")
	} else {
		ui.Warn("Found %d issue(s)", issues)
	}
	return nil
}
