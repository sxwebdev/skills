package cmd

import (
	"github.com/urfave/cli/v3"
)

func PromptCmd() *cli.Command {
	return &cli.Command{
		Name:  "prompt",
		Usage: "Prompt generation utilities",
		Commands: []*cli.Command{
			PromptBuildCmd(),
		},
	}
}
