package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/gitutil"
	"github.com/sxwebdev/skills/internal/installer"
	"github.com/sxwebdev/skills/internal/prompt"
	"github.com/sxwebdev/skills/internal/registry"
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

	// Group by (source, ref) so each distinct ref is fetched at the right ref.
	type srcKey struct{ repo, ref string }
	bySource := map[srcKey][]string{}
	for name, skill := range selected {
		key := srcKey{skill.Repo, skill.Ref}
		bySource[key] = append(bySource[key], name)
	}

	var updated, upToDate, newInstalled, removed int
	for key, names := range bySource {
		src, err := source.Parse(key.repo)
		if err != nil {
			ui.Warn("Skipping %s: %v", key.repo, err)
			continue
		}
		src.Ref = key.ref

		ui.Info("Checking %s…", src.Alias)

		// Cheap change detection (GitHub tree-SHA, one request, no download).
		// It can only clear a skill whose stored hash is the SAME kind; mixed
		// kinds (e.g. a legacy SHA1 baseline) fall through to a clone+compare so
		// an unchanged skill is never reinstalled.
		peek, peekKind, canPeek := source.PeekFolderHashes(ctx, src)
		var maybeChanged []string
		for _, name := range names {
			skill := cfg.Skills[name]
			if canPeek && skill.HashKind == peekKind {
				if cur, found := peek[normalizeFolder(skill.PathInRepo)]; found && cur == skill.FolderHash {
					upToDate++
					continue
				}
			}
			maybeChanged = append(maybeChanged, name)
		}

		// Discovery (offer new skills / removal of vanished ones) prompts the
		// user, so it runs only for an interactive, unfiltered update without
		// --yes. With --yes (or in CI) it is skipped so the run stays
		// non-interactive and never blocks waiting for input.
		discover := prompt.IsInteractive() && len(nameFilter) == 0 && !cmd.Bool("yes")

		if len(maybeChanged) == 0 && !discover {
			continue // everything verified unchanged via the cheap peek
		}

		// A clone is needed (to reinstall, to compare a non-peekable hash, or to
		// scan for new skills). Fetch lazily and once per source; remember a
		// failure so we don't re-attempt (and re-warn) for the same source.
		var fetched *source.Fetched
		fetchFailed := false
		ensureFetched := func() *source.Fetched {
			if fetched == nil && !fetchFailed {
				f, ferr := source.Find(key.repo).Fetch(ctx, src)
				if ferr != nil {
					ui.Warn("Failed to fetch %s: %v", src.Alias, ferr)
					fetchFailed = true
					return nil
				}
				fetched = &f
			}
			return fetched
		}

		// Skills whose folder is gone from the source (deleted or renamed).
		var missing []string
		for _, name := range maybeChanged {
			skill := cfg.Skills[name]
			if ensureFetched() == nil {
				break
			}

			srcDir := filepath.Join(fetched.Dir, skill.PathInRepo)
			if _, statErr := os.Stat(srcDir); os.IsNotExist(statErr) {
				ui.Warn("%s: no longer exists in source", name)
				missing = append(missing, name)
				continue
			}

			// Compare in the skill's stored kind so identical content is never
			// reported as an update; record the source's native baseline.
			newHash, newKind, err := fetched.FolderHash(skill.PathInRepo)
			if err != nil {
				ui.Warn("%s: %v", name, err)
				continue
			}
			changed, cerr := folderChanged(skill, srcDir, newHash, newKind)
			if cerr != nil {
				ui.Warn("%s: %v", name, cerr)
				continue
			}

			if !changed {
				// Silently upgrade the baseline (e.g. SHA1 → tree-SHA) so the
				// next update can use the cheap peek. Not reported as an update.
				if newHash != skill.FolderHash || newKind != skill.HashKind {
					skill.FolderHash = newHash
					skill.HashKind = newKind
					cfg.Skills[name] = skill
				}
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
				resolved, aerr := resolveAgents(cmd, cfg)
				if aerr != nil {
					ui.Warn("%s: %v", name, aerr)
					continue
				}
				agentList = resolved
			}
			mode, err := installer.InstallSkill(installer.InstallOpts{
				Name:        name,
				SrcDir:      srcDir,
				Agents:      agentList,
				ProjectRoot: skill.Project,
				ForceCopy:   skill.Mode == config.ModeCopy,
			})
			if err != nil {
				ui.Warn("%s: install failed: %v", name, err)
				continue
			}

			skill.FolderHash = newHash
			skill.HashKind = newKind
			skill.Mode = mode
			// Record the agents the skill was actually linked into. For a legacy
			// entry with no stored agents this is the freshly resolved list;
			// without it, a later `remove` could not unlink them (orphaned links).
			skill.Agents = agentList
			skill.UpdatedAt = time.Now().UTC()
			cfg.Skills[name] = skill
			updated++
			ui.Success("%s updated", name)
		}

		if discover {
			newInstalled += discoverNewSkills(cmd, cfg, ensureFetched, src, key.repo, projectRoot, dryRun)
			removed += offerRemoveMissing(cfg, missing, dryRun)
		}

		if fetched != nil {
			fetched.Cleanup()
		}
	}

	if !dryRun {
		if err := cfg.Save(); err != nil {
			return err
		}
	}
	ui.Info("\n%d updated, %d up to date, %d new, %d removed", updated, upToDate, newInstalled, removed)
	return nil
}

// discoverNewSkills scans the fetched source for skills not present in the
// config and offers to install the ones the user selects. Returns how many were
// installed (or, in dry-run, would be installed). Interactive only — the caller
// gates this on prompt.IsInteractive().
func discoverNewSkills(cmd *cli.Command, cfg *config.Config, ensureFetched func() *source.Fetched,
	src source.Source, repoKey, projectRoot string, dryRun bool) int {
	fetched := ensureFetched()
	if fetched == nil {
		return 0
	}

	discovered, err := registry.ScanRepo(fetched.Dir)
	if err != nil {
		ui.Warn("scan %s: %v", src.Alias, err)
		return 0
	}

	var newSkills []registry.DiscoveredSkill
	for _, d := range discovered {
		if _, exists := cfg.Skills[d.Name]; !exists {
			newSkills = append(newSkills, d)
		}
	}
	if len(newSkills) == 0 {
		return 0
	}

	if dryRun {
		for _, s := range newSkills {
			ui.Info("+ %s (new, would install)", s.Name)
		}
		return len(newSkills)
	}

	ui.Info("Found %d new skill(s) in %s", len(newSkills), src.Alias)
	selected, err := prompt.SelectSkills(newSkills)
	if err != nil {
		ui.Warn("%v", err)
		return 0
	}
	if len(selected) == 0 {
		return 0
	}

	agentList, err := resolveAgents(cmd, cfg)
	if err != nil {
		ui.Warn("%v", err)
		return 0
	}

	// The source already has installed skills, so it is registered; ensure it
	// regardless in case the registry entry was pruned.
	if _, exists := cfg.Repos[repoKey]; !exists {
		cfg.Repos[repoKey] = config.RepoInfo{Alias: src.Alias, AddedAt: time.Now().UTC()}
	}

	// Every skill in `selected` is, by construction, absent from cfg.Skills
	// (newSkills was filtered on that above), so no cross-repo duplicate guard is
	// needed here — unlike `add`, which can be pointed at an already-installed name.
	installed := 0
	for _, skill := range selected {
		if err := installAndRecord(cfg, fetched, src, repoKey, skill, agentList, projectRoot, false); err != nil {
			ui.Warn("Failed to install %s: %v", skill.Name, err)
			continue
		}
		ui.Success("%s installed", skill.Name)
		installed++
	}
	return installed
}

// offerRemoveMissing offers to remove skills whose folder vanished from the
// source (deleted or renamed). Returns how many were removed (or, in dry-run,
// would be removed). Interactive only — the caller gates this on
// prompt.IsInteractive().
func offerRemoveMissing(cfg *config.Config, missing []string, dryRun bool) int {
	if len(missing) == 0 {
		return 0
	}
	if dryRun {
		for _, name := range missing {
			ui.Info("- %s (missing in source, would remove)", name)
		}
		return len(missing)
	}

	removed := 0
	for _, name := range missing {
		skill, exists := cfg.Skills[name]
		if !exists {
			continue
		}
		ok, err := prompt.Confirm(fmt.Sprintf("Skill %q no longer exists in the source. Remove it?", name), false)
		if err != nil {
			ui.Warn("%v", err)
			continue
		}
		if !ok {
			continue
		}
		if err := removeOne(cfg, name, skill); err != nil {
			ui.Warn("Failed to remove %s: %v", name, err)
			continue
		}
		ui.Success("%s removed", name)
		removed++
	}
	return removed
}

// folderChanged reports whether the on-disk skill folder differs from the
// recorded baseline, compared in the SAME hash kind that was stored. When the
// stored kind matches the freshly computed kind we compare directly; otherwise
// we recompute the stored kind from disk so a legacy baseline (SHA1) is never
// mistaken for a change just because the new baseline is a tree-SHA.
func folderChanged(skill config.SkillInfo, srcDir, newHash, newKind string) (bool, error) {
	if skill.HashKind == newKind {
		return newHash != skill.FolderHash, nil
	}
	switch skill.HashKind {
	case config.HashKindSHA1, "":
		h, err := gitutil.ComputeFolderHash(srcDir)
		if err != nil {
			return false, err
		}
		return h != skill.FolderHash, nil
	default:
		// Unknown stored kind we can't recompute → treat as changed (reinstall).
		return true, nil
	}
}

// normalizeFolder converts a stored skill path to the slash-form key used by
// source.PeekFolderHashes.
func normalizeFolder(p string) string {
	return strings.TrimSuffix(filepath.ToSlash(p), "/")
}
