package cmd

import (
	"context"
	"path/filepath"
	"slices"
	"time"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/installer"
	"github.com/sxwebdev/skills/internal/source"
	"github.com/sxwebdev/skills/internal/ui"
	"github.com/urfave/cli/v3"
)

func UpdateCmd() *cli.Command {
	return &cli.Command{
		Name:      "update",
		Aliases:   []string{"upgrade", "check"},
		Usage:     "Update installed skills from their sources",
		ArgsUsage: "[skills...]",
		Flags: []cli.Flag{
			globalFlag(), agentFlag(), yesFlag(),
			&cli.BoolFlag{Name: "dry-run", Usage: "Show what would change without applying"},
		},
		Action: runUpdate,
	}
}

func runUpdate(ctx context.Context, cmd *cli.Command) error {
	projectRoot, err := resolveScope(cmd)
	if err != nil {
		return err
	}

	cfg, err := config.LoadOrCreate()
	if err != nil {
		return err
	}
	dryRun := cmd.Bool("dry-run")
	nameFilter := cmd.Args().Slice()

	// Select in-scope skills, optionally filtered by name.
	selected := map[string]config.SkillInfo{}
	for name, skill := range cfg.Skills {
		if projectRoot != "" && skill.Project != projectRoot {
			continue
		}
		if len(nameFilter) > 0 && !slices.Contains(nameFilter, name) {
			continue
		}
		selected[name] = skill
	}
	if len(selected) == 0 {
		ui.Info("No skills to update.")
		return nil
	}

	// Group by source key so each source is fetched once.
	bySource := map[string][]string{}
	for name, skill := range selected {
		bySource[skill.Repo] = append(bySource[skill.Repo], name)
	}

	var updated, upToDate int
	for repoKey, names := range bySource {
		src, err := source.Parse(repoKey)
		if err != nil {
			ui.Warn("Skipping %s: %v", repoKey, err)
			continue
		}
		// Reuse the pinned ref of the first skill in the group.
		src.Ref = selected[names[0]].Ref

		ui.Info("Checking %s…", src.Alias)
		fetched, err := source.Find(repoKey).Fetch(ctx, src)
		if err != nil {
			ui.Warn("Failed to fetch %s: %v", src.Alias, err)
			continue
		}

		for _, name := range names {
			skill := cfg.Skills[name]
			newHash, err := fetched.FolderHash(skill.PathInRepo)
			if err != nil {
				ui.Warn("%s: %v", name, err)
				continue
			}
			if newHash == skill.FolderHash && fetched.HashKind == skill.HashKind {
				upToDate++
				continue
			}
			if dryRun {
				ui.Info("~ %s (would update)", name)
				updated++
				continue
			}

			agentList := skill.Agents
			if len(agentList) == 0 {
				agentList, _ = resolveAgents(cmd, cfg)
			}
			mode, err := installer.InstallSkill(installer.InstallOpts{
				Name:        name,
				SrcDir:      filepath.Join(fetched.Dir, skill.PathInRepo),
				Agents:      agentList,
				ProjectRoot: skill.Project,
				ForceCopy:   skill.Mode == config.ModeCopy,
			})
			if err != nil {
				ui.Warn("%s: install failed: %v", name, err)
				continue
			}

			skill.FolderHash = newHash
			skill.HashKind = fetched.HashKind
			skill.Mode = mode
			skill.UpdatedAt = time.Now().UTC()
			cfg.Skills[name] = skill
			updated++
			ui.Success("%s updated", name)
		}
		fetched.Cleanup()
	}

	if !dryRun {
		if err := cfg.Save(); err != nil {
			return err
		}
	}
	ui.Info("\n%d updated, %d up to date", updated, upToDate)
	return nil
}
