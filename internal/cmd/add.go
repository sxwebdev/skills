package cmd

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/installer"
	"github.com/sxwebdev/skills/internal/prompt"
	"github.com/sxwebdev/skills/internal/registry"
	"github.com/sxwebdev/skills/internal/source"
	"github.com/sxwebdev/skills/internal/ui"
	"github.com/urfave/cli/v3"
)

func AddCmd() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Aliases:   []string{"a", "install", "i"},
		Usage:     "Add skills from a source (GitHub/GitLab/git/well-known/local)",
		ArgsUsage: "<source>",
		Flags: []cli.Flag{
			globalFlag(), agentFlag(), skillFlag(), yesFlag(), copyFlag(), allFlag(),
			&cli.BoolFlag{Name: "list", Aliases: []string{"l"}, Usage: "List skills in the source without installing"},
		},
		Action: runAdd,
	}
}

func runAdd(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() < 1 {
		return fmt.Errorf("usage: skills add <source>")
	}

	projectRoot, err := resolveScope(cmd)
	if err != nil {
		return err
	}

	cfg, err := config.LoadOrCreate()
	if err != nil {
		return err
	}
	agentList, err := resolveAgents(cmd, cfg)
	if err != nil {
		return err
	}

	raw := cmd.Args().First()
	src, err := source.Parse(raw)
	if err != nil {
		return err
	}

	ui.Info("Resolving %s (%s)…", src.Alias, src.Kind)
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
		ui.Warn("No skills found in %s", src.Alias)
		return nil
	}

	// Narrow by --skill or the source's @skill filter.
	wanted := cmd.StringSlice("skill")
	if src.SkillFilter != "" {
		wanted = append(wanted, src.SkillFilter)
	}
	if len(wanted) > 0 {
		skills = filterByNames(skills, wanted)
		if len(skills) == 0 {
			return fmt.Errorf("no matching skills for %v", wanted)
		}
	}

	if cmd.Bool("list") {
		ui.Heading(fmt.Sprintf("%d skill(s) in %s:", len(skills), src.Alias))
		for _, s := range skills {
			ui.Skill(s.Name, s.Description, "")
		}
		return nil
	}

	ui.Info("Found %d skill(s)", len(skills))

	selected := skills
	if !cmd.Bool("all") && !cmd.Bool("yes") && len(wanted) == 0 {
		selected, err = prompt.SelectSkills(skills)
		if err != nil {
			return err
		}
	}
	if len(selected) == 0 {
		ui.Info("No skills selected.")
		return nil
	}

	repoKey := src.CloneURL
	if src.Kind == source.KindLocal {
		repoKey = src.LocalDir
	}

	// Register the source so list/update/remove/doctor can group by it.
	if _, exists := cfg.Repos[repoKey]; !exists {
		cfg.Repos[repoKey] = config.RepoInfo{Alias: src.Alias, AddedAt: time.Now().UTC()}
	}

	installed := 0
	for _, skill := range selected {
		if existing, exists := cfg.Skills[skill.Name]; exists && existing.Repo != repoKey {
			ui.Warn("Skill %q already installed from %s, skipping", skill.Name, existing.Repo)
			continue
		}

		if err := installAndRecord(cfg, &fetched, src, repoKey, skill, agentList, projectRoot, cmd.Bool("copy")); err != nil {
			ui.Warn("Failed to install %s: %v", skill.Name, err)
			continue
		}
		ui.Success("%s", skill.Name)
		installed++
	}

	if err := cfg.Save(); err != nil {
		return err
	}
	ui.Info("Done! %d skill(s) installed.", installed)
	return nil
}

// installAndRecord installs one discovered skill and records it in cfg.Skills.
// It is the install+record step shared by `add` and `update` (new-skill
// discovery). The caller is responsible for the cross-repo duplicate guard and
// for persisting cfg via cfg.Save.
func installAndRecord(cfg *config.Config, fetched *source.Fetched, src source.Source, repoKey string,
	skill registry.DiscoveredSkill, agentList []string, projectRoot string, forceCopy bool) error {
	mode, err := installer.InstallSkill(installer.InstallOpts{
		Name:        skill.Name,
		SrcDir:      skill.AbsPath,
		Agents:      agentList,
		ProjectRoot: projectRoot,
		ForceCopy:   forceCopy,
	})
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}

	hash, hashKind, err := fetched.FolderHash(skill.PathInRepo)
	if err != nil {
		return fmt.Errorf("compute hash: %w", err)
	}

	now := time.Now().UTC()
	cfg.Skills[skill.Name] = config.SkillInfo{
		Repo:        repoKey,
		Description: skill.Description,
		PathInRepo:  skill.PathInRepo,
		FolderHash:  hash,
		HashKind:    hashKind,
		Agents:      slices.Clone(agentList),
		Mode:        mode,
		Ref:         src.Ref,
		Subpath:     src.Subpath,
		InstalledAt: now,
		UpdatedAt:   now,
		Project:     projectRoot,
	}
	return nil
}

func filterByNames(skills []registry.DiscoveredSkill, names []string) []registry.DiscoveredSkill {
	var out []registry.DiscoveredSkill
	for _, s := range skills {
		if slices.Contains(names, s.Name) {
			out = append(out, s)
		}
	}
	return out
}
