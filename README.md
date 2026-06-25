# skills

[![Go Reference](https://pkg.go.dev/badge/github.com/sxwebdev/skills.svg)](https://pkg.go.dev/github.com/sxwebdev/skills)
[![Go Version](https://img.shields.io/badge/go-1.26-blue)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/sxwebdev/skills)](https://goreportcard.com/report/github.com/sxwebdev/skills)
[![License](https://img.shields.io/github/license/sxwebdev/skills)](LICENSE)

A CLI tool for managing AI agent skills from Git repositories, well-known endpoints, and local paths.

Skills are stored in `~/.agents/skills/` and linked into agent-specific directories (e.g. `~/.claude/skills/`). Multiple agents are supported and auto-detected. Both global and per-project installations are tracked in a single, shareable config at `~/.skills/config.json`.

## Installation

```bash
go install github.com/sxwebdev/skills/cmd/skills@latest
```

## Quick Start

```bash
# Add skills from a source (interactive selection)
skills add owner/repo

# List installed skills
skills list

# Update all skills
skills update
```

No `init` step is required — the config is created automatically on first install.

## Commands

| Command                | Aliases             | Description                                         |
| ---------------------- | ------------------- | --------------------------------------------------- |
| `add <source>`         | `a`, `install`, `i` | Add skills from a source                            |
| `use <source>[@skill]` |                     | Print a skill's prompt to stdout without installing |
| `find [query]`         | `search`, `f`, `s`  | Search installed skills locally                     |
| `list`                 | `ls`                | List installed skills, grouped by source            |
| `update [skills...]`   | `upgrade`, `check`  | Update installed skills                             |
| `remove [skills...]`   | `rm`, `r`           | Remove skills (or all from a source)                |
| `init [name]`          |                     | Scaffold a new `SKILL.md` template                  |
| `doctor`               |                     | Diagnose installation issues                        |
| `prompt build <name>`  |                     | Generate an authoring prompt for a new skill        |

### `skills add <source>`

Resolves the source, discovers skills (directories with `SKILL.md` under `.agents/skills/`, `skills/`, or `.<agent>/skills/`), shows an interactive multi-select, and installs the chosen skills.

```bash
skills add owner/repo                 # GitHub shorthand
skills add owner/repo@my-skill        # only that skill
skills add github:owner/repo          # explicit GitHub
skills add gitlab:group/sub/repo      # GitLab (incl. subgroups)
skills add https://git.example.com/o/r.git   # generic git (Gitea, OneDev, ...)
skills add ./local-skills             # local path (no network)
skills add owner/repo --all           # install all without prompting
skills add owner/repo --list          # just list skills in the source
skills add owner/repo -a cursor -g    # install to the cursor agent, globally
skills add owner/repo --copy          # copy into agent dirs instead of symlinking
```

**Sources:** GitHub, GitLab (incl. self-hosted via `/-/tree/` URLs), generic git over HTTPS/SSH (Gitea, OneDev, …), RFC 8615 well-known endpoints (`/.well-known/agent-skills/index.json`), and local directories. Content is fetched with a shallow `git clone` (one packfile — far faster than many per-file HTTP requests); for GitHub a single git-tree-SHA request is also recorded so `update` can detect changes without re-cloning.

### `skills use <source>[@<skill>]`

Prints a skill's `SKILL.md` to stdout without installing it — pipe it straight to an agent.

```bash
skills use owner/repo@my-skill
skills use owner/repo@my-skill | claude
```

### `skills list` / `skills find`

```bash
skills list                # grouped by source, with status (symlink/copy, ok/missing)
skills list --json
skills find pdf            # substring search over installed skills
```

### `skills update`

Updates installed skills from their sources. For GitHub, change detection uses a single git-tree-SHA request and only changed repos are re-cloned; for other sources it clones and compares a content hash.

In an interactive terminal, an unfiltered `skills update` (no skill names, no `--yes`) also reconciles the source: it offers to install skills that have newly appeared in the source and to remove skills whose folder has disappeared from it (e.g. deleted or renamed). Non-interactively or with `--yes` it only updates the already-installed skills — a vanished skill is reported but never auto-removed. `--dry-run` previews everything (`~` updated, `+` new, `-` removed) without writing.

```bash
skills update                  # update + offer new/removed skills (interactive)
skills update my-skill         # update a specific skill only (no discovery)
skills update --dry-run        # show what would change
skills update --yes            # non-interactive: update existing only
```

### `skills remove`

```bash
skills remove my-skill                 # remove one (cleans symlink or copy)
skills remove a b c -y                  # remove several, no prompt
skills remove --all                     # remove everything in scope
skills remove owner/repo                # remove all skills from a source
skills remove owner/repo --keep-skills  # unregister the source, keep skills
```

### `skills init`

Scaffolds a `SKILL.md` template at `skills/<name>/SKILL.md` for authoring a new skill. For a richer, Claude-ready authoring prompt, use `skills prompt build <name>`.

## Agents

A configurable registry supports several agents (e.g. `claude-code`/`claude`, `cursor`, `codex`, `opencode`, `gemini`). Installed agents are auto-detected. Target specific agents with `-a/--agent` (repeatable). Each agent's skills live under `~/.<agent>/skills` (global) or `<project>/.<agent>/skills` (per-project).

## Per-Project Skills

By default skills are installed globally. Use `--local`/`--project` to scope to a project, or `-g/--global` to force global.

```bash
skills add owner/repo --local              # current project (auto-detect via .git)
skills add owner/repo --project /path/to/p
skills list --local
```

When installed per-project, skills are stored in `<project>/.agents/skills/` with links at `<project>/.<agent>/skills/`. The global config at `~/.skills/config.json` tracks all skills — both global and per-project.

## How It Works

1. **Config** at `~/.skills/config.json` tracks sources, installed skills (global and per-project), and agents. Created lazily on first install; migrated automatically across schema versions.
2. **Skills** are copied to `~/.agents/skills/<name>/` (global) or `<project>/.agents/skills/<name>/` (per-project).
3. **Links** are created for each agent: a relative symlink by default, falling back to a copy when symlinking isn't possible (or with `--copy`).
4. **Content** is fetched with a shallow `git clone`. **Updates** compare the GitHub git-tree SHA where available (one request, no clone unless something changed), otherwise a deterministic SHA1 folder hash — only changed skills are reinstalled.

## Security

Untrusted skill names and descriptions are stripped of terminal escape sequences before display, skill names are validated against path traversal, and well-known archives are extracted with path-safety, link rejection, and size/count limits.

## Skill Repository Structure

```text
repo/
├── .agents/skills/my-skill/SKILL.md
└── skills/another-skill/SKILL.md
```

Each skill directory must contain a `SKILL.md` with YAML frontmatter:

```markdown
---
name: my-skill
description: Does something useful
---

Skill instructions here...
```

## License

MIT
