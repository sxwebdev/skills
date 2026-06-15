package cmd

import (
	"os"
	"path/filepath"

	"github.com/sxwebdev/skills/internal/config"
)

// scopeOf returns a human-readable scope for a skill ("global" or project path).
func scopeOf(skill config.SkillInfo) string {
	if skill.Project == "" {
		return "global"
	}
	return skill.Project
}

// skillLinkIssues returns human-readable problems with a skill's install
// directory and its agent links (symlink or copy). An empty slice means the
// skill is healthy. Shared by `list` and `doctor` so both agree on what
// "healthy" means.
func skillLinkIssues(name string, skill config.SkillInfo) []string {
	skillDir := filepath.Join(config.ResolveSkillsInstallDir(skill.Project), name)
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return []string{"directory missing at " + skillDir}
	}

	var issues []string
	for _, agent := range skill.Agents {
		agentDir := config.ResolveAgentSkillsDir(skill.Project, agent)
		if agentDir == "" {
			issues = append(issues, "unknown agent "+agent)
			continue
		}
		linkPath := filepath.Join(agentDir, name)

		if skill.Mode == config.ModeCopy {
			if _, err := os.Stat(linkPath); err != nil {
				issues = append(issues, "missing copy ("+agent+")")
			}
			continue
		}

		target, err := os.Readlink(linkPath)
		if err != nil {
			issues = append(issues, "no symlink ("+agent+")")
			continue
		}
		abs := target
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(agentDir, target)
		}
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			issues = append(issues, "broken symlink ("+agent+" → "+target+")")
		}
	}
	return issues
}
