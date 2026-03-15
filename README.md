# skills

A CLI tool for managing AI agent skills from Git repositories.

Skills are stored in `~/.agents/skills/` and symlinked to agent-specific directories (e.g., `~/.claude/skills/` for Claude Code).

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
skills repo add owner/repo                        # shorthand for GitHub
skills repo add https://github.com/owner/repo.git # full URL
skills repo add owner/repo --all                  # install all skills without prompting
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

## How It Works

1. **Config** is stored at `~/.skills/config.json` — tracks repos, installed skills, and agent integrations.
2. **Skills** are copied to `~/.agents/skills/<name>/`.
3. **Symlinks** are created for each configured agent (e.g., `~/.claude/skills/<name>` → `../../.agents/skills/<name>`).
4. **Updates** use folder content hashing (SHA1) to detect changes — only modified skills are reinstalled.

## Skill Repository Structure

A repository should contain skills in one of these locations:

```
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
