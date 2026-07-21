package registry

import (
	"fmt"
	"io/fs"
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

// ScanRepo scans a cloned repo directory for skills. It walks the whole repo
// tree and treats any directory that contains a SKILL.md as a skill, wherever
// it lives — skills/, .agents/skills/, .<agent>/skills/ (e.g. .claude/skills/)
// as well as arbitrary nested layouts such as questdb/skills' questdb/<skill>/
// SKILL.md. VCS metadata (.git) is skipped, and once a skill is found its own
// subdirectories are treated as its resources rather than nested skills.
func ScanRepo(repoDir string) ([]DiscoveredSkill, error) {
	var skills []DiscoveredSkill

	err := filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		// Never descend into VCS metadata or dependency directories.
		if path != repoDir && (d.Name() == ".git" || d.Name() == "node_modules") {
			return fs.SkipDir
		}

		data, readErr := os.ReadFile(filepath.Join(path, "SKILL.md"))
		if readErr != nil {
			if os.IsNotExist(readErr) {
				return nil // not a skill directory; keep descending
			}
			return fmt.Errorf("read SKILL.md: %w", readErr)
		}

		name, description := parseFrontmatter(data, d.Name())
		rel, _ := filepath.Rel(repoDir, path)
		skills = append(skills, DiscoveredSkill{
			Name:        name,
			Description: description,
			PathInRepo:  rel,
			AbsPath:     path,
		})
		// A skill's own subdirectories are its resources, not nested skills.
		return fs.SkipDir
	})
	if err != nil {
		return nil, err
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
