package config

import (
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

// SkillsInstallDir returns ~/.agents/skills/
func SkillsInstallDir() string {
	return filepath.Join(HomeDir(), ".agents", "skills")
}

// AgentSkillsDir returns the skills directory for a given agent.
func AgentSkillsDir(agent string) string {
	switch agent {
	case "claude-code":
		return filepath.Join(HomeDir(), ".claude", "skills")
	default:
		return ""
	}
}
