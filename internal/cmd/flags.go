package cmd

import (
	"fmt"

	"github.com/sxwebdev/skills/internal/agents"
	"github.com/sxwebdev/skills/internal/config"
	"github.com/urfave/cli/v3"
)

// Shared flag builders, so the command surface stays consistent.

func globalFlag() cli.Flag {
	return &cli.BoolFlag{Name: "global", Aliases: []string{"g"}, Usage: "Operate on global skills (~)"}
}

func agentFlag() cli.Flag {
	return &cli.StringSliceFlag{Name: "agent", Aliases: []string{"a"}, Usage: "Target agent(s) (repeatable)"}
}

func skillFlag() cli.Flag {
	return &cli.StringSliceFlag{Name: "skill", Aliases: []string{"s"}, Usage: "Specific skill name(s)"}
}

func yesFlag() cli.Flag {
	return &cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "Skip confirmation prompts"}
}

func jsonFlag() cli.Flag {
	return &cli.BoolFlag{Name: "json", Usage: "Output in JSON format"}
}

func copyFlag() cli.Flag {
	return &cli.BoolFlag{Name: "copy", Usage: "Copy skills into agent dirs instead of symlinking"}
}

func allFlag() cli.Flag {
	return &cli.BoolFlag{Name: "all", Usage: "Apply to all (skip selection prompt)"}
}

// resolveScope determines the project root, reconciling -g/--global (force
// global) with the root --local/--project flags.
func resolveScope(cmd *cli.Command) (string, error) {
	if cmd.Bool("global") {
		return "", nil
	}
	return resolveProject(cmd)
}

// resolveAgents resolves the target agents for a command. Precedence:
// explicit -a/--agent, then the config's agent list, then auto-detected agents,
// finally claude-code. Returns canonical agent names.
func resolveAgents(cmd *cli.Command, cfg *config.Config) ([]string, error) {
	if requested := cmd.StringSlice("agent"); len(requested) > 0 {
		names := make([]string, 0, len(requested))
		for _, r := range requested {
			a, ok := agents.Get(r)
			if !ok {
				return nil, fmt.Errorf("unknown agent %q (known: %v)", r, agents.Names())
			}
			names = append(names, a.Name)
		}
		return names, nil
	}
	if len(cfg.Agents) > 0 {
		return cfg.Agents, nil
	}
	if detected := agents.Detect(); len(detected) > 0 {
		names := make([]string, len(detected))
		for i, a := range detected {
			names[i] = a.Name
		}
		return names, nil
	}
	return []string{"claude-code"}, nil
}
