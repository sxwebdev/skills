package main

import (
	"context"
	"fmt"
	"os"

	commands "github.com/sxwebdev/skills/internal/cmd"
	"github.com/urfave/cli/v3"
)

var version = "dev"

func main() {
	app := &cli.Command{
		Name:    "skills",
		Usage:   "AI agent skills manager",
		Version: version,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "local",
				Aliases: []string{"l"},
				Usage:   "Use project-local skills (auto-detect project root from cwd)",
			},
			&cli.StringFlag{
				Name:    "project",
				Aliases: []string{"p"},
				Usage:   "Use skills in a specific project path",
			},
		},
		Commands: []*cli.Command{
			commands.InitCmd(),
			commands.RepoCmd(),
			commands.ListCmd(),
			commands.UpdateCmd(),
			commands.RemoveCmd(),
			commands.DoctorCmd(),
			commands.PromptCmd(),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
