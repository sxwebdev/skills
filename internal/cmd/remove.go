package cmd

import (
	"context"
	"fmt"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/installer"
	"github.com/sxwebdev/skills/internal/prompt"
	"github.com/sxwebdev/skills/internal/source"
	"github.com/sxwebdev/skills/internal/ui"
	"github.com/urfave/cli/v3"
)

func RemoveCmd() *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Aliases:   []string{"rm", "r"},
		Usage:     "Remove installed skills (or all skills from a source)",
		ArgsUsage: "[skills... | <source>]",
		Flags: []cli.Flag{
			globalFlag(), agentFlag(), skillFlag(), yesFlag(), allFlag(),
			&cli.BoolFlag{Name: "keep-skills", Usage: "With a source arg: unregister it but keep installed skills"},
		},
		Action: runRemove,
	}
}

func runRemove(_ context.Context, cmd *cli.Command) error {
	projectRoot, err := resolveScope(cmd)
	if err != nil {
		return err
	}

	cfg, err := config.LoadOrCreate()
	if err != nil {
		return err
	}

	args := cmd.Args().Slice()
	args = append(args, cmd.StringSlice("skill")...)

	// Source-scoped removal: a single arg that matches a registered source.
	if len(args) == 1 {
		if repoKey, ok := matchSource(cfg, args[0]); ok {
			return removeSource(cmd, cfg, repoKey)
		}
	}

	// Determine which skills to remove.
	var names []string
	switch {
	case cmd.Bool("all"):
		for name, skill := range cfg.Skills {
			if inScope(skill, projectRoot) {
				names = append(names, name)
			}
		}
	case len(args) > 0:
		names = args
	default:
		return fmt.Errorf("usage: skills remove <skill>... (or --all)")
	}

	if len(names) == 0 {
		ui.Info("No skills to remove.")
		return nil
	}

	if !cmd.Bool("yes") {
		ok, err := prompt.Confirm(fmt.Sprintf("Remove %d skill(s)?", len(names)), false)
		if err != nil {
			return err
		}
		if !ok {
			ui.Info("Cancelled.")
			return nil
		}
	}

	removed := 0
	for _, name := range names {
		skill, exists := cfg.Skills[name]
		if !exists {
			ui.Warn("Skill %q is not installed", name)
			continue
		}
		if err := installer.RemoveSkill(name, skill.Agents, skill.Project); err != nil {
			ui.Warn("Failed to remove %s: %v", name, err)
			continue
		}
		delete(cfg.Skills, name)
		pruneRepo(cfg, skill.Repo)
		ui.Success("%s removed", name)
		removed++
	}

	if err := cfg.Save(); err != nil {
		return err
	}
	ui.Info("%d skill(s) removed.", removed)
	return nil
}

func removeSource(cmd *cli.Command, cfg *config.Config, repoKey string) error {
	var toRemove []string
	for name, skill := range cfg.Skills {
		if skill.Repo == repoKey {
			toRemove = append(toRemove, name)
		}
	}
	alias := repoKey
	if info, ok := cfg.Repos[repoKey]; ok {
		alias = info.Alias
	}

	if !cmd.Bool("keep-skills") && len(toRemove) > 0 {
		if !cmd.Bool("yes") {
			ok, err := prompt.Confirm(fmt.Sprintf("Remove %d skill(s) from %s?", len(toRemove), alias), true)
			if err != nil {
				return err
			}
			if !ok {
				ui.Info("Cancelled.")
				return nil
			}
		}
		for _, name := range toRemove {
			skill := cfg.Skills[name]
			if err := installer.RemoveSkill(name, skill.Agents, skill.Project); err != nil {
				ui.Warn("Failed to remove %s: %v", name, err)
				continue
			}
			delete(cfg.Skills, name)
			ui.Success("%s removed", name)
		}
	}

	delete(cfg.Repos, repoKey)
	if err := cfg.Save(); err != nil {
		return err
	}
	ui.Info("Source %s unregistered.", alias)
	return nil
}

// matchSource reports whether arg refers to a registered source, returning its key.
func matchSource(cfg *config.Config, arg string) (string, bool) {
	if _, ok := cfg.Repos[arg]; ok {
		return arg, true
	}
	if src, err := source.Parse(arg); err == nil {
		key := src.CloneURL
		if src.Kind == source.KindLocal {
			key = src.LocalDir
		}
		if _, ok := cfg.Repos[key]; ok {
			return key, true
		}
		// Match by alias too.
		for k, info := range cfg.Repos {
			if info.Alias == src.Alias {
				return k, true
			}
		}
	}
	return "", false
}

func inScope(skill config.SkillInfo, projectRoot string) bool {
	return projectRoot == "" || skill.Project == projectRoot
}

// pruneRepo drops a source from the registry once no skills reference it.
func pruneRepo(cfg *config.Config, repoKey string) {
	for _, skill := range cfg.Skills {
		if skill.Repo == repoKey {
			return
		}
	}
	delete(cfg.Repos, repoKey)
}
