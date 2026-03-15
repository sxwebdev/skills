package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/gitutil"
	"github.com/sxwebdev/skills/internal/installer"
	"github.com/sxwebdev/skills/internal/prompt"
	"github.com/sxwebdev/skills/internal/registry"
	"github.com/urfave/cli/v3"
)

func RepoAddCmd() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Add a skill repository",
		ArgsUsage: "<url>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "all",
				Aliases: []string{"a"},
				Usage:   "Install all skills without prompting",
			},
			&cli.BoolFlag{
				Name:  "skip-install",
				Usage: "Register repo without installing skills",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.NArg() < 1 {
				return fmt.Errorf("usage: skills repo add <url>")
			}

			projectRoot, err := resolveProject(cmd)
			if err != nil {
				return err
			}

			rawURL := cmd.Args().First()
			url, err := NormalizeRepoURL(rawURL)
			if err != nil {
				return err
			}

			cfg := config.MustLoad()

			// Check if repo already registered
			if _, exists := cfg.Repos[url]; exists {
				fmt.Printf("Repository %s is already registered.\n", AliasFromURL(url))
			}

			// Register repo
			cfg.Repos[url] = config.RepoInfo{
				Alias:   AliasFromURL(url),
				AddedAt: time.Now().UTC(),
			}

			if cmd.Bool("skip-install") {
				if err := cfg.Save(); err != nil {
					return err
				}
				fmt.Println("✓ Repository registered:", AliasFromURL(url))
				return nil
			}

			// Ask to download
			if !cmd.Bool("all") {
				ok, err := prompt.Confirm("Download skills from this repository?", true)
				if err != nil {
					return err
				}
				if !ok {
					if err := cfg.Save(); err != nil {
						return err
					}
					fmt.Println("✓ Repository registered (skills not installed):", AliasFromURL(url))
					return nil
				}
			}

			// Clone repo
			fmt.Printf("Cloning %s...\n", AliasFromURL(url))
			tmpDir, err := gitutil.CloneShallow(ctx, url)
			if err != nil {
				return fmt.Errorf("clone failed: %w", err)
			}
			defer func() {
				if err := os.RemoveAll(tmpDir); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to clean up temp dir %s: %v\n", tmpDir, err)
				}
			}()

			// Scan for skills
			skills, err := registry.ScanRepo(tmpDir)
			if err != nil {
				return fmt.Errorf("scan repo: %w", err)
			}
			if len(skills) == 0 {
				fmt.Println("No skills found in repository.")
				if err := cfg.Save(); err != nil {
					return err
				}
				return nil
			}

			fmt.Printf("Found %d skill(s)\n", len(skills))

			// Select skills
			selected := skills
			if !cmd.Bool("all") {
				selected, err = prompt.SelectSkills(skills)
				if err != nil {
					return err
				}
			}

			if len(selected) == 0 {
				fmt.Println("No skills selected.")
				if err := cfg.Save(); err != nil {
					return err
				}
				return nil
			}

			// Install selected skills
			for _, skill := range selected {
				// Check name conflict
				if existing, exists := cfg.Skills[skill.Name]; exists && existing.Repo != url {
					fmt.Printf("⚠ Skill %q already installed from %s, skipping\n", skill.Name, AliasFromURL(existing.Repo))
					continue
				}

				if err := installer.InstallSkill(skill.Name, skill.AbsPath, cfg.Agents, projectRoot); err != nil {
					fmt.Printf("⚠ Failed to install %s: %v\n", skill.Name, err)
					continue
				}

				hash, err := gitutil.ComputeFolderHash(skill.AbsPath)
				if err != nil {
					fmt.Printf("⚠ Failed to compute hash for %s: %v\n", skill.Name, err)
					continue
				}
				now := time.Now().UTC()
				cfg.Skills[skill.Name] = config.SkillInfo{
					Repo:        url,
					PathInRepo:  skill.PathInRepo,
					FolderHash:  hash,
					InstalledAt: now,
					UpdatedAt:   now,
					Project:     projectRoot,
				}
				fmt.Printf("  ✓ %s\n", skill.Name)
			}

			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Printf("Done! %d skill(s) installed.\n", len(selected))
			return nil
		},
	}
}
