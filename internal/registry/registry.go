package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DiscoveredSkill represents a skill found in a cloned repository.
type DiscoveredSkill struct {
	Name        string
	Description string
	PathInRepo  string // e.g. "skills/find-skills"
	AbsPath     string // absolute path in temp clone
}

// ScanRepo scans a cloned repo directory for skills. It looks for skills in
// skills/, .agents/skills/, and any .<agent>/skills/ container (e.g.
// .claude/skills/, .cursor/skills/) so on-disk discovery matches the GitHub
// fast-path and the documented layout.
func ScanRepo(repoDir string) ([]DiscoveredSkill, error) {
	var skills []DiscoveredSkill

	searchPaths := []string{
		filepath.Join(repoDir, "skills"),
		filepath.Join(repoDir, ".agents", "skills"),
	}
	// Add any top-level dotdir that holds a skills/ subdir (.claude, .cursor, …).
	if entries, err := os.ReadDir(repoDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), ".") && e.Name() != ".git" && e.Name() != ".agents" {
				searchPaths = append(searchPaths, filepath.Join(repoDir, e.Name(), "skills"))
			}
		}
	}

	for _, searchPath := range searchPaths {
		entries, err := os.ReadDir(searchPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read dir %s: %w", searchPath, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			skillDir := filepath.Join(searchPath, entry.Name())
			skillMD := filepath.Join(skillDir, "SKILL.md")

			data, err := os.ReadFile(skillMD)
			if err != nil {
				if os.IsNotExist(err) {
					continue // not a skill directory
				}
				return nil, fmt.Errorf("read SKILL.md: %w", err)
			}

			name, description := parseFrontmatter(data, entry.Name())

			rel, _ := filepath.Rel(repoDir, skillDir)
			skills = append(skills, DiscoveredSkill{
				Name:        name,
				Description: description,
				PathInRepo:  rel,
				AbsPath:     skillDir,
			})
		}
	}

	return skills, nil
}

// ReadSkillMeta reads a skill's name and description from <dir>/SKILL.md,
// falling back to the directory base name if the frontmatter is missing.
func ReadSkillMeta(dir string) (name, description string, err error) {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return "", "", err
	}
	name, description = parseFrontmatter(data, filepath.Base(dir))
	return name, description, nil
}

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func parseFrontmatter(data []byte, fallbackName string) (name, description string) {
	content := string(data)

	if !strings.HasPrefix(content, "---") {
		return fallbackName, ""
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return fallbackName, ""
	}

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
		return fallbackName, ""
	}

	if fm.Name == "" {
		fm.Name = fallbackName
	}
	return fm.Name, fm.Description
}
