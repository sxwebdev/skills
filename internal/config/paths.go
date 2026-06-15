package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sxwebdev/skills/internal/agents"
)

func HomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic("cannot determine home directory: " + err.Error())
	}
	return home
}

// ConfigDir returns ~/.skills/
func ConfigDir() string {
	return filepath.Join(HomeDir(), ".skills")
}

// ConfigFile returns ~/.skills/config.json
func ConfigFile() string {
	return filepath.Join(ConfigDir(), "config.json")
}

// SkillsInstallDir returns the global skills directory ~/.agents/skills/
func SkillsInstallDir() string {
	return filepath.Join(HomeDir(), ".agents", "skills")
}

// AgentSkillsDir returns the global agent skills directory for the named agent.
func AgentSkillsDir(agent string) string {
	return agents.ResolveGlobalDir(agent)
}

// ResolveSkillsInstallDir returns the skills install dir for the given project.
// If projectRoot is empty, returns the global directory.
func ResolveSkillsInstallDir(projectRoot string) string {
	if projectRoot == "" {
		return SkillsInstallDir()
	}
	return filepath.Join(projectRoot, ".agents", "skills")
}

// ResolveAgentSkillsDir returns the agent skills dir for the given project.
// If projectRoot is empty, returns the global directory.
func ResolveAgentSkillsDir(projectRoot, agent string) string {
	if projectRoot == "" {
		return agents.ResolveGlobalDir(agent)
	}
	return agents.ResolveProjectDir(agent, projectRoot)
}

// FindProjectRoot walks up from cwd looking for a .git directory.
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		gitDir := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no git repository found (searched up from cwd)")
		}
		dir = parent
	}
}
