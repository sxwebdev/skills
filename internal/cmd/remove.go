package cmd

import (
	"context"
	"fmt"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/installer"
	"github.com/sxwebdev/skills/internal/prompt"
	"github.com/urfave/cli/v3"
)

func RemoveCmd() *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Usage:     "Remove an installed skill",
		ArgsUsage: "<skill-name>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Skip confirmation",
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() < 1 {
				return fmt.Errorf("usage: skills remove <skill-name>")
			}

			name := cmd.Args().First()
			cfg := config.MustLoad()

			if _, exists := cfg.Skills[name]; !exists {
				return fmt.Errorf("skill %q is not installed", name)
			}

			if !cmd.Bool("force") {
				ok, err := prompt.Confirm(fmt.Sprintf("Remove skill %q?", name), false)
				if err != nil {
					return err
				}
				if !ok {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			if err := installer.RemoveSkill(name, cfg.Agents); err != nil {
				return fmt.Errorf("remove skill: %w", err)
			}

			delete(cfg.Skills, name)
			if err := cfg.Save(); err != nil {
				return err
			}

			fmt.Printf("✓ Skill %q removed\n", name)
			return nil
		},
	}
}
