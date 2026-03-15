package cmd

import (
	"github.com/urfave/cli/v3"
)

func RepoCmd() *cli.Command {
	return &cli.Command{
		Name:  "repo",
		Usage: "Manage skill repositories",
		Commands: []*cli.Command{
			RepoAddCmd(),
			RepoListCmd(),
			RepoRemoveCmd(),
		},
	}
}
