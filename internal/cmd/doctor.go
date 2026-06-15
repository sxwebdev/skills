package cmd

import (
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
		scope := scopeOf(skill)

		for _, issue := range skillLinkIssues(name, skill) {
			ui.Error("Skill %q [%s]: %s", name, scope, issue)
			issues++
		}

		if skill.Repo != "" && skill.Project == "" {
			if _, exists := cfg.Repos[skill.Repo]; !exists {
				ui.Warn("Skill %q [%s]: source %s not registered (orphaned)", name, scope, skill.Repo)
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
