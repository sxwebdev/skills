package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"
)

const skillPromptTemplate = `You are a skill generator for a Claude Code agent skill library.

## Task

Generate a complete skill named ` + "`{SKILL_NAME}`" + ` and save all files to ` + "`skills/{SKILL_NAME}/`" + `.

## What is a skill

A skill is a structured knowledge base that Claude Code loads when invoked. It consists of:
- A main ` + "`SKILL.md`" + ` entry point with metadata and instructions
- Supporting ` + "`.md`" + ` documentation files
- A ` + "`templates/`" + ` subdirectory with reusable code snippets (if the skill involves code generation)

## Required file: SKILL.md

Every skill must have ` + "`skills/{SKILL_NAME}/SKILL.md`" + ` with this exact frontmatter format:

` + "```" + `
---
name: {SKILL_NAME}
description: {One sentence describing what this skill does and when it triggers}
user-invocable: false
---

# {Skill Title}

## When to use
[Conditions that trigger this skill. Be specific about keywords, file types, or patterns in user requests.]

## How to proceed
[Step-by-step instructions Claude should follow when this skill is invoked.]

## Key principles
[Core rules and constraints to follow.]
` + "```" + `

## File structure rules

1. **All files are Markdown** (` + "`.md`" + ` extension), including code templates.
2. **Code templates** go in ` + "`skills/{SKILL_NAME}/templates/<category>/`" + ` as ` + "`.md`" + ` files. Each template file contains one or more related code blocks with explanation.
3. **Documentation files** (architecture, checklist, overview, etc.) go directly in ` + "`skills/{SKILL_NAME}/`" + `.
4. **Placeholders** for user-supplied values use ` + "`{UPPER_SNAKE_CASE}`" + ` format (e.g. ` + "`{APP_NAME}`" + `, ` + "`{MODULE}`" + `, ` + "`{ENTITY}`" + `). Document all placeholders used.

## File naming conventions

| Purpose | Example filename |
|---|---|
| Main entry point | ` + "`SKILL.md`" + ` |
| Tech stack / dependencies | ` + "`stack-overview.md`" + ` |
| Directory layout | ` + "`project-structure.md`" + ` |
| Design patterns | ` + "`architecture.md`" + ` |
| Step-by-step checklist | ` + "`{entity}-checklist.md`" + ` |
| Code template | ` + "`templates/{category}/{topic}.md`" + ` |

## Template file format

Each file in ` + "`templates/`" + ` should follow this structure:

` + "```" + `
# {Topic Title}

Brief explanation of what this template does and where it lives in the project.

## {Filename or Section}

` + "````{language}" + `
// actual code here
// use {PLACEHOLDER} for user-supplied values
` + "````" + `

> Note: Any important usage notes or caveats.
` + "```" + `

## What to generate

Based on the skill name ` + "`{SKILL_NAME}`" + ` and the following description:

> {SKILL_DESCRIPTION}

Generate ALL of the following:

1. ` + "`skills/{SKILL_NAME}/SKILL.md`" + ` — metadata + instructions
2. ` + "`skills/{SKILL_NAME}/stack-overview.md`" + ` — dependencies and installation (if applicable)
3. ` + "`skills/{SKILL_NAME}/architecture.md`" + ` — key patterns and design decisions
4. ` + "`skills/{SKILL_NAME}/project-structure.md`" + ` — directory layout and bootstrap steps (if applicable)
5. Any additional documentation files that make sense for this skill
6. All template files under ` + "`skills/{SKILL_NAME}/templates/`" + ` organized by category

## Quality criteria

- **Complete**: Every referenced file must be created. No placeholders like "add content here".
- **Consistent**: Placeholders must be uniform across all files.
- **Actionable**: Instructions in SKILL.md must be precise enough that Claude can follow them without ambiguity.
- **Self-contained**: The skill should work without external references that may not be available.

Write all files now using the Write tool.
`

func PromptBuildCmd() *cli.Command {
	return &cli.Command{
		Name:      "build",
		Usage:     "Generate a prompt for creating a new skill",
		ArgsUsage: "<name>",
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() < 1 {
				return fmt.Errorf("usage: skills prompt build <name>")
			}

			name := cmd.Args().First()
			result := strings.ReplaceAll(skillPromptTemplate, "{SKILL_NAME}", name)
			fmt.Print(result)
			return nil
		},
	}
}
