package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/registry"
	"github.com/sxwebdev/skills/internal/ui"
	"github.com/urfave/cli/v3"
)

type foundSkill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Scope       string `json:"scope"`
}

func FindCmd() *cli.Command {
	return &cli.Command{
		Name:      "find",
		Aliases:   []string{"search", "f", "s"},
		Usage:     "Search installed skills locally",
		ArgsUsage: "[query]",
		Flags:     []cli.Flag{jsonFlag()},
		Action:    runFind,
	}
}

func runFind(_ context.Context, cmd *cli.Command) error {
	cfg, err := config.LoadOrCreate()
	if err != nil {
		return err
	}
	query := strings.ToLower(strings.Join(cmd.Args().Slice(), " "))

	var results []foundSkill
	for name, skill := range cfg.Skills {
		desc := ""
		dir := filepath.Join(config.ResolveSkillsInstallDir(skill.Project), name)
		if _, d, err := registry.ReadSkillMeta(dir); err == nil {
			desc = d
		}
		if query != "" && !strings.Contains(strings.ToLower(name), query) && !strings.Contains(strings.ToLower(desc), query) {
			continue
		}
		scope := "global"
		if skill.Project != "" {
			scope = skill.Project
		}
		results = append(results, foundSkill{Name: name, Description: desc, Source: skill.Repo, Scope: scope})
	}

	slices.SortFunc(results, func(a, b foundSkill) int { return strings.Compare(a.Name, b.Name) })

	if cmd.Bool("json") {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	if len(results) == 0 {
		ui.Info("No matching skills installed.")
		return nil
	}
	ui.Heading(fmt.Sprintf("%d skill(s):", len(results)))
	for _, r := range results {
		ui.Skill(r.Name, r.Description, r.Scope)
	}
	return nil
}
