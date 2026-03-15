package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/gitutil"
	"github.com/sxwebdev/skills/internal/installer"
	"github.com/urfave/cli/v3"
)

func UpdateCmd() *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "Update installed skills from their repositories",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "skill",
				Aliases: []string{"s"},
				Usage:   "Update only a specific skill",
			},
			&cli.BoolFlag{
				Name:  "dry-run",
				Usage: "Show what would change without applying",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			projectRoot, err := resolveProject(cmd)
			if err != nil {
				return err
			}

			cfg := config.MustLoad()
			dryRun := cmd.Bool("dry-run")
			filterSkill := cmd.String("skill")

			// Filter skills by project if --local or --project
			skills := cfg.Skills
			if projectRoot != "" {
				skills = make(map[string]config.SkillInfo)
				for name, skill := range cfg.Skills {
					if skill.Project == projectRoot {
						skills[name] = skill
					}
				}
			}

			if len(skills) == 0 {
				fmt.Println("No skills installed.")
				return nil
			}

			// Group skills by repo
			repoSkills := make(map[string][]string)
			for name, skill := range skills {
				if filterSkill != "" && name != filterSkill {
					continue
				}
				repoSkills[skill.Repo] = append(repoSkills[skill.Repo], name)
			}

			if filterSkill != "" && len(repoSkills) == 0 {
				return fmt.Errorf("skill %q not found", filterSkill)
			}

			var updated, upToDate int

			for repoURL, skillNames := range repoSkills {
				fmt.Printf("Checking %s...\n", AliasFromURL(repoURL))

				tmpDir, err := gitutil.CloneShallow(ctx, repoURL)
				if err != nil {
					fmt.Printf("⚠ Failed to clone %s: %v\n", AliasFromURL(repoURL), err)
					continue
				}

				for _, name := range skillNames {
					skill := cfg.Skills[name]
					srcDir := filepath.Join(tmpDir, skill.PathInRepo)

					if _, err := os.Stat(srcDir); os.IsNotExist(err) {
						fmt.Printf("  ⚠ %s: no longer exists in repo\n", name)
						continue
					}

					newHash, err := gitutil.ComputeFolderHash(srcDir)
					if err != nil {
						fmt.Printf("  ⚠ %s: hash error: %v\n", name, err)
						continue
					}

					if newHash == skill.FolderHash {
						upToDate++
						continue
					}

					if dryRun {
						fmt.Printf("  ~ %s (would update)\n", name)
						updated++
						continue
					}

					if err := installer.InstallSkill(name, srcDir, cfg.Agents, skill.Project); err != nil {
						fmt.Printf("  ⚠ %s: install failed: %v\n", name, err)
						continue
					}

					skill.FolderHash = newHash
					skill.UpdatedAt = time.Now().UTC()
					cfg.Skills[name] = skill
					updated++
					fmt.Printf("  ✓ %s updated\n", name)
				}

				if err := os.RemoveAll(tmpDir); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to clean up temp dir %s: %v\n", tmpDir, err)
				}
			}

			if !dryRun {
				if err := cfg.Save(); err != nil {
					return err
				}
			}

			fmt.Printf("\n%d updated, %d up to date\n", updated, upToDate)
			return nil
		},
	}
}
