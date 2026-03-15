package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/urfave/cli/v3"
)

func ListCmd() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List installed skills",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "json",
				Usage: "Output in JSON format",
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			projectRoot, err := resolveProject(cmd)
			if err != nil {
				return err
			}

			cfg := config.MustLoad()

			// Filter skills by project if --local or --project is set
			filtered := cfg.Skills
			if projectRoot != "" {
				filtered = make(map[string]config.SkillInfo)
				for name, skill := range cfg.Skills {
					if skill.Project == projectRoot {
						filtered[name] = skill
					}
				}
			}

			if cmd.Bool("json") {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(filtered)
			}

			if len(filtered) == 0 {
				fmt.Println("No skills installed. Use 'skills repo add <url>' to get started.")
				return nil
			}

			for name, skill := range filtered {
				status := "✓"
				skillDir := filepath.Join(config.ResolveSkillsInstallDir(skill.Project), name)
				if _, err := os.Stat(skillDir); os.IsNotExist(err) {
					status = "⚠ missing"
				}

				// Check symlinks
				for _, agent := range cfg.Agents {
					agentDir := config.ResolveAgentSkillsDir(skill.Project, agent)
					if agentDir == "" {
						continue
					}
					linkPath := filepath.Join(agentDir, name)
					if target, err := os.Readlink(linkPath); err != nil {
						status = "⚠ no symlink"
					} else {
						abs := target
						if !filepath.IsAbs(abs) {
							abs = filepath.Join(agentDir, target)
						}
						if _, err := os.Stat(abs); os.IsNotExist(err) {
							status = "⚠ broken symlink"
						}
					}
				}

				scope := "global"
				if skill.Project != "" {
					scope = skill.Project
				}

				fmt.Printf("  %s %s  (%s)  [%s]  updated: %s\n",
					status, name, AliasFromURL(skill.Repo), scope,
					skill.UpdatedAt.Format("2006-01-02"),
				)
			}
			return nil
		},
	}
}
