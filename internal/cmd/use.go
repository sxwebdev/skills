package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sxwebdev/skills/internal/registry"
	"github.com/sxwebdev/skills/internal/source"
	"github.com/urfave/cli/v3"
)

func UseCmd() *cli.Command {
	return &cli.Command{
		Name:      "use",
		Usage:     "Print a skill's prompt to stdout without installing (pipeable to an agent)",
		ArgsUsage: "<source>[@<skill>]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "skill", Aliases: []string{"s"}, Usage: "Skill name to use"},
		},
		Action: runUse,
	}
}

func runUse(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() < 1 {
		return fmt.Errorf("usage: skills use <source>[@<skill>]")
	}

	raw := cmd.Args().First()
	src, err := source.Parse(raw)
	if err != nil {
		return err
	}

	// -s/--skill overrides the source's own @<skill> selector.
	skillName := cmd.String("skill")
	if skillName == "" {
		skillName = src.SkillFilter
	}

	fetched, err := source.Find(raw).Fetch(ctx, src)
	if err != nil {
		return fmt.Errorf("fetch source: %w", err)
	}
	defer fetched.Cleanup()

	skills, err := registry.ScanRepo(fetched.Dir)
	if err != nil {
		return fmt.Errorf("scan source: %w", err)
	}
	if len(skills) == 0 {
		return fmt.Errorf("no skills found in %s", src.Alias)
	}

	// Pick the skill: by name, or the only one available.
	var absPath string
	switch {
	case skillName != "":
		for _, s := range skills {
			if s.Name == skillName {
				absPath = s.AbsPath
			}
		}
		if absPath == "" {
			return fmt.Errorf("skill %q not found in %s", skillName, src.Alias)
		}
	case len(skills) == 1:
		absPath = skills[0].AbsPath
	default:
		names := make([]string, len(skills))
		for i, s := range skills {
			names[i] = s.Name
		}
		return fmt.Errorf("multiple skills found; specify one with @<skill>: %s", strings.Join(names, ", "))
	}

	data, err := os.ReadFile(filepath.Join(absPath, "SKILL.md"))
	if err != nil {
		return fmt.Errorf("read SKILL.md: %w", err)
	}

	// stdout is the raw prompt (pipe-safe). fatih/color auto-disables on pipes,
	// but the prompt body is printed verbatim regardless.
	out := string(data)
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	_, err = io.WriteString(os.Stdout, out)
	return err
}
