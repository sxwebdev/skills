package cmd

import (
	"context"
	"fmt"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/installer"
	"github.com/sxwebdev/skills/internal/prompt"
	"github.com/urfave/cli/v3"
)

func RepoRemoveCmd() *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Usage:     "Remove a repository and optionally its skills",
		ArgsUsage: "<url>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "keep-skills",
				Usage: "Keep installed skills, only unregister the repository",
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() < 1 {
				return fmt.Errorf("usage: skills repo remove <url>")
			}

			rawURL := cmd.Args().First()
			url, err := NormalizeRepoURL(rawURL)
			if err != nil {
				return err
			}

			cfg := config.MustLoad()

			if _, exists := cfg.Repos[url]; !exists {
				return fmt.Errorf("repository %s not found", rawURL)
			}

			alias := cfg.Repos[url].Alias

			if !cmd.Bool("keep-skills") {
				// Find and remove all skills from this repo
				var toRemove []string
				for name, skill := range cfg.Skills {
					if skill.Repo == url {
						toRemove = append(toRemove, name)
					}
				}

				if len(toRemove) > 0 {
					ok, err := prompt.Confirm(
						fmt.Sprintf("Remove %d skill(s) from %s?", len(toRemove), alias),
						true,
					)
					if err != nil {
						return err
					}
					if ok {
						for _, name := range toRemove {
							skill := cfg.Skills[name]
							if err := installer.RemoveSkill(name, cfg.Agents, skill.Project); err != nil {
								fmt.Printf("⚠ Failed to remove %s: %v\n", name, err)
								continue
							}
							delete(cfg.Skills, name)
							fmt.Printf("  ✓ Removed %s\n", name)
						}
					}
				}
			}

			delete(cfg.Repos, url)
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Printf("✓ Repository %s removed\n", alias)
			return nil
		},
	}
}
