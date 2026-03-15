package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/urfave/cli/v3"
)

// resolveProject determines the project root from CLI flags.
// Returns "" for global, or an absolute path for project-local.
func resolveProject(cmd *cli.Command) (string, error) {
	root := cmd.Root()

	projectPath := root.String("project")
	if projectPath != "" {
		abs, err := filepath.Abs(projectPath)
		if err != nil {
			return "", fmt.Errorf("resolve project path: %w", err)
		}
		return abs, nil
	}

	if root.Bool("local") {
		root, err := config.FindProjectRoot()
		if err != nil {
			return "", err
		}
		return root, nil
	}

	return "", nil
}
