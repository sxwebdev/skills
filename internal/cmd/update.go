package cmd

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/gitutil"
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

	// Group by (source, ref) so each distinct ref is fetched at the right ref.
	type srcKey struct{ repo, ref string }
	bySource := map[srcKey][]string{}
	for name, skill := range selected {
		key := srcKey{skill.Repo, skill.Ref}
		bySource[key] = append(bySource[key], name)
	}

	var updated, upToDate int
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
		if len(maybeChanged) == 0 {
			continue // everything verified unchanged via the cheap peek
		}

		// A clone is needed (to reinstall, or to compare a non-peekable hash).
		// Fetch lazily and once per source.
		var fetched *source.Fetched
		for _, name := range maybeChanged {
			skill := cfg.Skills[name]
			if fetched == nil {
				f, ferr := source.Find(key.repo).Fetch(ctx, src)
				if ferr != nil {
					ui.Warn("Failed to fetch %s: %v", src.Alias, ferr)
					break
				}
				fetched = &f
			}

			srcDir := filepath.Join(fetched.Dir, skill.PathInRepo)
			if _, statErr := os.Stat(srcDir); os.IsNotExist(statErr) {
				ui.Warn("%s: no longer exists in source", name)
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
			skill.UpdatedAt = time.Now().UTC()
			cfg.Skills[name] = skill
			updated++
			ui.Success("%s updated", name)
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
	ui.Info("\n%d updated, %d up to date", updated, upToDate)
	return nil
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
