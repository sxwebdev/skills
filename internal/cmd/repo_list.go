package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/urfave/cli/v3"
)

func RepoListCmd() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List registered repositories",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "json",
				Usage: "Output in JSON format",
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			cfg := config.MustLoad()

			if cmd.Bool("json") {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(cfg.Repos)
			}

			if len(cfg.Repos) == 0 {
				fmt.Println("No repositories registered. Use 'skills repo add <url>' to add one.")
				return nil
			}

			for url, repo := range cfg.Repos {
				// Count skills from this repo
				count := 0
				for _, s := range cfg.Skills {
					if s.Repo == url {
						count++
					}
				}
				fmt.Printf("  %s (%d skills)\n", repo.Alias, count)
			}
			return nil
		},
	}
}
