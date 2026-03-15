package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/urfave/cli/v3"
)

func InitCmd() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize skills configuration",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Overwrite existing config",
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			force := cmd.Bool("force")
			projectRoot, err := resolveProject(cmd)
			if err != nil {
				return err
			}

			// Check if config already exists (global config only)
			if _, err := os.Stat(config.ConfigFile()); err == nil && !force {
				return fmt.Errorf("config already exists at %s (use --force to overwrite)", config.ConfigFile())
			}

			cfg := config.NewDefault()
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			// Create directories
			dirs := []string{config.ResolveSkillsInstallDir(projectRoot)}
			for _, agent := range cfg.Agents {
				if dir := config.ResolveAgentSkillsDir(projectRoot, agent); dir != "" {
					dirs = append(dirs, dir)
				}
			}
			for _, dir := range dirs {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Errorf("create dir %s: %w", dir, err)
				}
			}

			fmt.Println("✓ Config created at", config.ConfigFile())
			fmt.Println("✓ Skills directory:", config.ResolveSkillsInstallDir(projectRoot))
			if projectRoot != "" {
				fmt.Println("✓ Project:", projectRoot)
			}
			return nil
		},
	}
}
