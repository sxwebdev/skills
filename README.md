# skills

[![Go Reference](https://pkg.go.dev/badge/github.com/sxwebdev/skills.svg)](https://pkg.go.dev/github.com/sxwebdev/skills)
[![Go Version](https://img.shields.io/badge/go-1.26-blue)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/sxwebdev/skills)](https://goreportcard.com/report/github.com/sxwebdev/skills)
[![License](https://img.shields.io/github/license/sxwebdev/skills)](LICENSE)

A CLI tool for managing AI agent skills from Git repositories.

Skills are stored in `~/.agents/skills/` and symlinked to agent-specific directories (e.g., `~/.claude/skills/` for Claude Code). Supports both global and per-project skill installations.

## Installation

```bash
go install github.com/sxwebdev/skills/cmd/skills@latest
```

## Quick Start

```bash
# Initialize config and directories
skills init

# Add a repository and install skills
skills repo add owner/repo

# List installed skills
skills list

# Update all skills
skills update
```

## Commands

### `skills init`

Creates the configuration file at `~/.skills/config.json` and sets up required directories.

```bash
skills init
skills init --force  # overwrite existing config
```

### `skills repo add <url>`

Adds a Git repository and offers to install skills from it. The tool clones the repo, scans for skills (directories containing `SKILL.md` in `.agents/skills/` or `skills/`), and presents an interactive multi-select prompt.

```bash
skills repo add owner/repo                         # shorthand for GitHub
skills repo add https://github.com/owner/repo.git  # full URL
skills repo add owner/repo --all                   # install all skills without prompting
skills repo add owner/repo --skip-install          # register repo only
```

### `skills repo list`

Lists all registered repositories with the number of installed skills.

```bash
skills repo list
skills repo list --json
```

### `skills repo remove <url>`

Removes a repository and optionally all skills installed from it.

```bash
skills repo remove owner/repo
skills repo remove owner/repo --keep-skills  # keep skills, only unregister repo
```

### `skills list`

Lists all installed skills with status indicators for missing directories or broken symlinks.

```bash
skills list
skills list --json
```

### `skills update`

Updates installed skills by comparing folder hashes with the latest version in their source repositories.

```bash
skills update                  # update all skills
skills update --skill my-skill # update a specific skill
skills update --dry-run        # show what would change
```

### `skills remove <name>`

Removes an installed skill, its directory, and all agent symlinks.

```bash
skills remove my-skill
skills remove my-skill --force  # skip confirmation
```

### `skills doctor`

Diagnoses issues: missing directories, broken symlinks, orphaned skills, and config inconsistencies.

```bash
skills doctor
```

### `skills prompt build <name>`

Generates a ready-to-use prompt for creating a new skill. The output can be copied into Claude Code or piped to a file.

```bash
skills prompt build my-skill          # print prompt to stdout
skills prompt build my-skill > p.md   # save to file
```

## Per-Project Skills

By default, skills are installed globally. Use `--local` or `--project` flags to install skills into a specific project instead.

```bash
# Install skills into the current project (auto-detects project root via .git)
skills repo add owner/repo --local

# Install skills into a specific project
skills repo add owner/repo --project /path/to/project

# List only skills for the current project
skills list --local

# Update only project-local skills
skills update --local

# Short flags work too
skills repo add owner/repo -l
skills repo add owner/repo -p /path/to/project
```

When installed per-project, skills are stored in `<project>/.agents/skills/` with symlinks at `<project>/.claude/skills/`. The global config at `~/.skills/config.json` tracks all skills — both global and per-project.

## How It Works

1. **Config** is stored at `~/.skills/config.json` — tracks repos, installed skills (global and per-project), and agent integrations.
2. **Skills** are copied to `~/.agents/skills/<name>/` (global) or `<project>/.agents/skills/<name>/` (per-project).
3. **Symlinks** are created for each configured agent (e.g., `~/.claude/skills/<name>` → `../../.agents/skills/<name>`).
4. **Updates** use folder content hashing (SHA1) to detect changes — only modified skills are reinstalled.

## Skill Repository Structure

A repository should contain skills in one of these locations:

```text
repo/
├── .agents/
│   └── skills/
│       ├── my-skill/
│       │   └── SKILL.md
│       └── another-skill/
│           └── SKILL.md
└── skills/
    └── yet-another/
        └── SKILL.md
```

Each skill directory must contain a `SKILL.md` file. Optional YAML frontmatter is used for metadata:

```markdown
---
name: my-skill
description: Does something useful
---

Skill instructions here...
```

## License

MIT
