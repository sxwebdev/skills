package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/ui"
	"github.com/urfave/cli/v3"
)

func ListCmd() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "List installed skills, grouped by source",
		Flags:   []cli.Flag{globalFlag(), agentFlag(), jsonFlag()},
		Action:  runList,
	}
}

func runList(_ context.Context, cmd *cli.Command) error {
	projectRoot, err := resolveScope(cmd)
	if err != nil {
		return err
	}

	cfg, err := config.LoadOrCreate()
	if err != nil {
		return err
	}

	filtered := make(map[string]config.SkillInfo)
	for name, skill := range cfg.Skills {
		if projectRoot != "" && skill.Project != projectRoot {
			continue
		}
		filtered[name] = skill
	}

	if cmd.Bool("json") {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(filtered)
	}

	if len(filtered) == 0 {
		ui.Info("No skills installed. Use 'skills add <source>' to get started.")
		return nil
	}

	// Group skill names by source.
	bySource := map[string][]string{}
	for name, skill := range filtered {
		bySource[skill.Repo] = append(bySource[skill.Repo], name)
	}

	for _, repoKey := range slices.Sorted(maps.Keys(bySource)) {
		alias := repoKey
		if info, ok := cfg.Repos[repoKey]; ok {
			alias = info.Alias
		}
		ui.Heading(alias)

		for _, name := range slices.Sorted(slices.Values(bySource[repoKey])) {
			skill := filtered[name]
			status := "ok"
			if issues := skillLinkIssues(name, skill); len(issues) > 0 {
				status = strings.Join(issues, ", ")
			}
			extra := fmt.Sprintf("[%s] %s · %s", scopeOf(skill), skill.Mode, status)
			ui.Skill(name, "", extra)
		}
	}
	return nil
}
