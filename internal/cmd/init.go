package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sxwebdev/skills/internal/sanitize"
	"github.com/sxwebdev/skills/internal/ui"
	"github.com/urfave/cli/v3"
)

const skillTemplate = `---
name: %s
description: One or two sentences describing what this skill does AND when it should trigger — include keywords, file types, and user-intent phrases.
---

# %s

## Overview
Explain what this skill helps accomplish and the reasoning behind the approach.

## Instructions
Step-by-step instructions in imperative form. Explain WHY each step matters.

## Examples
Provide at least one realistic input/output example.
`

func InitCmd() *cli.Command {
	return &cli.Command{
		Name:      "init",
		Usage:     "Scaffold a new SKILL.md template",
		ArgsUsage: "[name]",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "Overwrite an existing SKILL.md"},
		},
		Action: runInit,
	}
}

func runInit(_ context.Context, cmd *cli.Command) error {
	name := cmd.Args().First()
	if name == "" {
		name = "my-skill"
	}
	clean, err := sanitize.Name(name)
	if err != nil {
		return err
	}

	dir := filepath.Join("skills", clean)
	skillMD := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(skillMD); err == nil && !cmd.Bool("force") {
		return fmt.Errorf("%s already exists (use --force to overwrite)", skillMD)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create skill dir: %w", err)
	}
	content := fmt.Sprintf(skillTemplate, clean, clean)
	if err := os.WriteFile(skillMD, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write SKILL.md: %w", err)
	}

	ui.Success("Created %s", skillMD)
	ui.Info("Edit the description and instructions, then run: skills add ./%s", filepath.Dir(dir))
	return nil
}
