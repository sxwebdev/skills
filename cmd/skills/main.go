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
		Commands: []*cli.Command{
			commands.InitCmd(),
			commands.RepoCmd(),
			commands.ListCmd(),
			commands.UpdateCmd(),
			commands.RemoveCmd(),
			commands.DoctorCmd(),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
