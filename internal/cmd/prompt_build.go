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

A skill is structured knowledge that Claude Code loads progressively to help with specific tasks. Skills use a three-level loading system:

1. **Metadata** (name + description in YAML frontmatter) — always visible to Claude (~100 words). This is the primary triggering mechanism: Claude decides whether to invoke a skill based solely on its description.
2. **SKILL.md body** — loaded into context when the skill triggers. Keep under 500 lines. This is where the core instructions live.
3. **Bundled resources** — loaded on demand, unlimited size. These include scripts, reference docs, and assets that SKILL.md points to when needed.

### Skill anatomy

` + "```" + `
{SKILL_NAME}/
├── SKILL.md          # Entry point (required, <500 lines)
├── scripts/          # Executable code for deterministic/repetitive tasks
├── references/       # Documentation loaded into context as needed
└── assets/           # Files used in output (templates, icons, fonts)
` + "```" + `

Not every skill needs all directories — create only what the skill actually requires.

## Required file: SKILL.md

Every skill must have ` + "`skills/{SKILL_NAME}/SKILL.md`" + ` with YAML frontmatter and a markdown body.

### Frontmatter format

` + "```yaml" + `
---
name: {SKILL_NAME}
description: {Triggering description — see guidelines below}
user-invocable: false
---
` + "```" + `

### Writing the description (critical for triggering)

The description field is the PRIMARY mechanism that determines whether Claude invokes the skill. Claude tends to under-trigger skills, so a good description must be slightly "pushy":

- Include BOTH what the skill does AND specific keywords/contexts that should trigger it
- Mention file types, frameworks, user intent patterns, and adjacent concepts
- All "when to use" information belongs in the description, not in the body

**Bad**: "Helps with data visualization."
**Good**: "Build interactive data dashboards and charts. Use this skill whenever the user mentions dashboards, data visualization, plotting, charts, graphs, metrics displays, or wants to present data visually — even if they don't explicitly ask for a 'dashboard'."

### Body structure

` + "```markdown" + `
# {Skill Title}

## Overview
Brief explanation of what this skill helps accomplish and the reasoning behind the approach.
This context helps Claude understand the "why" and make better decisions in edge cases.

## Instructions
Step-by-step instructions in imperative form.
Explain WHY each step matters — this helps Claude generalize beyond the literal instructions.

## Domain variants (if applicable)
When the skill covers multiple frameworks or technologies, list them here with pointers
to the relevant reference file in references/. Claude reads only the one it needs.

Example:
- React: see references/react.md
- Vue: see references/vue.md

## Examples
Include at least one realistic input/output example so Claude understands the expected pattern.

**Example 1:**
Input: User asks "..."
Output: Claude does X, producing Y

## Output format (if applicable)
Define the expected output format explicitly. Use a template:
# [Title]
## Section 1
## Section 2

## Key principles
Core rules framed as reasoning, not commands. Explain consequences so Claude can
handle cases you didn't anticipate.
` + "```" + `

## Writing style

Follow these principles when writing skill content:

- **Imperative form**: "Run the test suite" not "You should run the test suite"
- **Explain WHY, not just WHAT**: Instead of "ALWAYS use prepared statements", write "Use prepared statements for database queries because string interpolation opens the door to SQL injection." Reasoning helps Claude handle edge cases intelligently.
- **Generalize**: Write skills that work across many prompts, not just narrow examples. Use theory of mind — think about what different users might ask.
- **Principle of Lack of Surprise**: A skill's behavior should match what a user would expect from its description. No hidden side effects.
- **Progressive disclosure**: Keep SKILL.md focused on the core workflow. Move heavy reference material into ` + "`references/`" + ` with clear pointers from SKILL.md about when to read each file.

## File structure rules

1. ` + "`SKILL.md`" + ` is the entry point and must stay under 500 lines. If approaching this limit, move detailed content into ` + "`references/`" + ` with clear pointers.
2. ` + "`scripts/`" + ` contains executable code (bash, python, etc.) for deterministic tasks that should not be reinvented each time.
3. ` + "`references/`" + ` contains documentation files loaded as needed. For files over 300 lines, include a table of contents at the top. When a skill spans multiple domains/frameworks, create separate files per variant (e.g. ` + "`references/react.md`" + `, ` + "`references/vue.md`" + `).
4. ` + "`assets/`" + ` contains output templates, icons, fonts — files copied or used in skill output.
5. **Placeholders** for user-supplied values use ` + "`{UPPER_SNAKE_CASE}`" + ` format (e.g. ` + "`{APP_NAME}`" + `, ` + "`{MODULE}`" + `). Document all placeholders used.
6. **All documentation and reference files are Markdown** (` + "`.md`" + ` extension). Code templates in ` + "`scripts/`" + ` use their native extension.

## What to generate

Based on the skill name ` + "`{SKILL_NAME}`" + ` and the following description:

> {SKILL_DESCRIPTION}

Generate the following (skip directories that don't apply):

1. ` + "`skills/{SKILL_NAME}/SKILL.md`" + ` — frontmatter with a high-quality triggering description + instructions under 500 lines, including at least one input/output example
2. ` + "`skills/{SKILL_NAME}/references/`" + ` — domain-specific documentation, organized by variant if multi-framework
3. ` + "`skills/{SKILL_NAME}/scripts/`" + ` — executable code for deterministic/repetitive operations
4. ` + "`skills/{SKILL_NAME}/assets/`" + ` — output templates and static files
5. Any additional files that make sense for the skill's domain

## Quality criteria

- **Complete**: Every file referenced in SKILL.md must exist. No placeholders like "add content here".
- **Consistent**: Placeholders must be uniform across all files.
- **Actionable**: Instructions must be precise enough for Claude to follow without ambiguity.
- **Self-contained**: The skill works without external references that may not be available.
- **Well-triggered**: The description clearly communicates when the skill should activate, including specific keywords, file types, and user intent patterns.
- **Progressive**: Heavy content lives in ` + "`references/`" + `, keeping SKILL.md under 500 lines.
- **Exemplified**: SKILL.md contains at least one realistic input/output example.
- **Reasoned**: Instructions explain WHY, not just WHAT — this helps Claude handle edge cases.

Write all files now using the Write tool.
`

func PromptBuildCmd() *cli.Command {
	return &cli.Command{
		Name:      "build",
		Usage:     "Generate a prompt for creating a new skill",
		ArgsUsage: "<name>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "description",
				Aliases: []string{"d"},
				Usage:   "Brief description of what the skill does",
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() < 1 {
				return fmt.Errorf("usage: skills prompt build <name> [-d description]")
			}

			name := cmd.Args().First()
			description := cmd.String("description")
			if description == "" {
				description = "[Describe what this skill does and when it should trigger]"
			}

			result := strings.ReplaceAll(skillPromptTemplate, "{SKILL_NAME}", name)
			result = strings.ReplaceAll(result, "{SKILL_DESCRIPTION}", description)
			fmt.Print(result)
			return nil
		},
	}
}
