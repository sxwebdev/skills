package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

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
			status := skillStatus(name, skill)
			extra := fmt.Sprintf("[%s] %s · %s", scopeOf(skill), skill.Mode, status)
			ui.Skill(name, "", extra)
		}
	}
	return nil
}

func scopeOf(skill config.SkillInfo) string {
	if skill.Project == "" {
		return "global"
	}
	return skill.Project
}

// skillStatus checks the install directory and agent links (symlink or copy).
func skillStatus(name string, skill config.SkillInfo) string {
	skillDir := filepath.Join(config.ResolveSkillsInstallDir(skill.Project), name)
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return "missing"
	}

	for _, agent := range skill.Agents {
		agentDir := config.ResolveAgentSkillsDir(skill.Project, agent)
		if agentDir == "" {
			continue
		}
		linkPath := filepath.Join(agentDir, name)

		if skill.Mode == config.ModeCopy {
			if _, err := os.Stat(linkPath); err != nil {
				return "missing copy (" + agent + ")"
			}
			continue
		}

		target, err := os.Readlink(linkPath)
		if err != nil {
			return "no symlink (" + agent + ")"
		}
		abs := target
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(agentDir, target)
		}
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			return "broken symlink (" + agent + ")"
		}
	}
	return "ok"
}
