package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/urfave/cli/v3"
)

func DoctorCmd() *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "Diagnose and report issues with skills installation",
		Action: func(_ context.Context, _ *cli.Command) error {
			issues := 0

			// Check config
			cfg, err := config.Load()
			if err != nil {
				fmt.Printf("✗ Config: %v\n\nFound 1 issue(s)\n", err)
				return nil
			}
			fmt.Println("✓ Config loaded")

			// Check skills directory
			if _, err := os.Stat(config.SkillsInstallDir()); os.IsNotExist(err) {
				fmt.Println("✗ Skills directory missing:", config.SkillsInstallDir())
				issues++
			} else {
				fmt.Println("✓ Skills directory exists")
			}

			// Check agent directories
			for _, agent := range cfg.Agents {
				dir := config.AgentSkillsDir(agent)
				if dir == "" {
					continue
				}
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					fmt.Printf("✗ Agent directory missing: %s (%s)\n", dir, agent)
					issues++
				} else {
					fmt.Printf("✓ Agent directory exists: %s\n", agent)
				}
			}

			// Check each skill
			for name, skill := range cfg.Skills {
				skillDir := filepath.Join(config.SkillsInstallDir(), name)

				// Check skill directory
				if _, err := os.Stat(skillDir); os.IsNotExist(err) {
					fmt.Printf("✗ Skill %q: directory missing at %s\n", name, skillDir)
					issues++
					continue
				}

				// Check repo is registered
				if _, exists := cfg.Repos[skill.Repo]; !exists {
					fmt.Printf("⚠ Skill %q: repo %s not registered (orphaned skill)\n", name, AliasFromURL(skill.Repo))
					issues++
				}

				// Check symlinks
				for _, agent := range cfg.Agents {
					agentDir := config.AgentSkillsDir(agent)
					if agentDir == "" {
						continue
					}
					linkPath := filepath.Join(agentDir, name)

					target, err := os.Readlink(linkPath)
					if err != nil {
						fmt.Printf("✗ Skill %q: symlink missing for %s\n", name, agent)
						issues++
						continue
					}

					// Resolve relative symlink
					abs := target
					if !filepath.IsAbs(abs) {
						abs = filepath.Join(agentDir, target)
					}
					if _, err := os.Stat(abs); os.IsNotExist(err) {
						fmt.Printf("✗ Skill %q: broken symlink for %s → %s\n", name, agent, target)
						issues++
					}
				}
			}

			// Check for orphaned directories in skills dir
			entries, err := os.ReadDir(config.SkillsInstallDir())
			if err == nil {
				for _, entry := range entries {
					if !entry.IsDir() {
						continue
					}
					if _, exists := cfg.Skills[entry.Name()]; !exists {
						fmt.Printf("⚠ Orphaned directory: %s\n", filepath.Join(config.SkillsInstallDir(), entry.Name()))
						issues++
					}
				}
			}

			fmt.Println()
			if issues == 0 {
				fmt.Println("No issues found!")
			} else {
				fmt.Printf("Found %d issue(s)\n", issues)
			}
			return nil
		},
	}
}
