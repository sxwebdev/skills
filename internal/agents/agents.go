// Package agents is a data-driven registry of AI coding agents that consume
// skills. Each agent declares where its skills live (globally and per-project)
// and how to detect whether it is installed. The installer and path resolution
// consume this registry instead of hardcoding a single agent.
//
// This package must NOT import internal/config to avoid an import cycle:
// config/paths.go delegates agent directory resolution here.
package agents

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Agent describes a single AI coding agent and where it stores skills.
type Agent struct {
	// Name is the canonical identifier, e.g. "claude-code".
	Name string
	// Aliases are alternate names accepted on the command line, e.g. "claude".
	Aliases []string
	// ProjectSkillsDir returns the skills directory inside a project root.
	ProjectSkillsDir func(projectRoot string) string
	// GlobalSkillsDir returns the user-global skills directory.
	GlobalSkillsDir func() string
	// Detect reports whether this agent appears to be installed.
	Detect func() bool
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// dotDirAgent builds an Agent whose skills live under "<dot>/skills"
// both globally (~/<dot>/skills) and per-project (<root>/<dot>/skills),
// detected by the presence of "~/<dot>" or a binary on PATH.
func dotDirAgent(name string, aliases []string, dot string, bins ...string) Agent {
	return Agent{
		Name:    name,
		Aliases: aliases,
		ProjectSkillsDir: func(projectRoot string) string {
			return filepath.Join(projectRoot, dot, "skills")
		},
		GlobalSkillsDir: func() string {
			home := homeDir()
			if home == "" {
				return ""
			}
			return filepath.Join(home, dot, "skills")
		},
		Detect: func() bool {
			if home := homeDir(); home != "" {
				if _, err := os.Stat(filepath.Join(home, dot)); err == nil {
					return true
				}
			}
			for _, bin := range bins {
				if _, err := exec.LookPath(bin); err == nil {
					return true
				}
			}
			return false
		},
	}
}

// registry holds the default agent set. Adding an agent is data-only.
var registry = []Agent{
	dotDirAgent("claude-code", []string{"claude"}, ".claude", "claude"),
	dotDirAgent("cursor", nil, ".cursor", "cursor"),
	dotDirAgent("codex", nil, ".codex", "codex"),
	dotDirAgent("opencode", nil, ".opencode", "opencode"),
	dotDirAgent("gemini", []string{"gemini-cli"}, ".gemini", "gemini"),
}

// All returns every registered agent.
func All() []Agent {
	out := make([]Agent, len(registry))
	copy(out, registry)
	return out
}

// Names returns the canonical names of all registered agents.
func Names() []string {
	names := make([]string, len(registry))
	for i, a := range registry {
		names[i] = a.Name
	}
	return names
}

// Get looks up an agent by canonical name or alias.
func Get(nameOrAlias string) (Agent, bool) {
	for _, a := range registry {
		if a.Name == nameOrAlias {
			return a, true
		}
		for _, alias := range a.Aliases {
			if alias == nameOrAlias {
				return a, true
			}
		}
	}
	return Agent{}, false
}

// Detect returns the agents that appear to be installed.
func Detect() []Agent {
	var out []Agent
	for _, a := range registry {
		if a.Detect != nil && a.Detect() {
			out = append(out, a)
		}
	}
	return out
}

// ResolveGlobalDir returns the global skills directory for the named agent,
// or "" if the agent is unknown.
func ResolveGlobalDir(nameOrAlias string) string {
	a, ok := Get(nameOrAlias)
	if !ok {
		return ""
	}
	return a.GlobalSkillsDir()
}

// ResolveProjectDir returns the per-project skills directory for the named
// agent, or "" if the agent is unknown.
func ResolveProjectDir(nameOrAlias, projectRoot string) string {
	a, ok := Get(nameOrAlias)
	if !ok {
		return ""
	}
	return a.ProjectSkillsDir(projectRoot)
}
