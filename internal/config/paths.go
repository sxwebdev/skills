package config

import (
	"fmt"
	"os"
	"path/filepath"
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

// AgentSkillsDir returns the global agent skills directory.
func AgentSkillsDir(agent string) string {
	switch agent {
	case "claude-code":
		return filepath.Join(HomeDir(), ".claude", "skills")
	default:
		return ""
	}
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
		return AgentSkillsDir(agent)
	}
	switch agent {
	case "claude-code":
		return filepath.Join(projectRoot, ".claude", "skills")
	default:
		return ""
	}
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
