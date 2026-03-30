package prompt

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/sxwebdev/skills/internal/registry"
	"golang.org/x/term"
)

// IsInteractive returns true if stdin is a terminal.
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// SelectSkills presents a multi-select prompt for skills.
// Returns the selected skills. In non-interactive mode returns all skills.
func SelectSkills(skills []registry.DiscoveredSkill) ([]registry.DiscoveredSkill, error) {
	if !IsInteractive() || len(skills) == 0 {
		return skills, nil
	}

	cols, rows, _ := term.GetSize(int(os.Stdout.Fd()))
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	// Prefix width: selector "> " + checkbox "• " = ~6 chars
	maxLabelWidth := cols - 6
	maxLabelWidth = max(maxLabelWidth, 20)

	options := make([]huh.Option[string], len(skills))
	for i, s := range skills {
		label := s.Name
		if s.Description != "" {
			label = fmt.Sprintf("%s — %s", s.Name, s.Description)
		}
		if len(label) > maxLabelWidth {
			label = label[:maxLabelWidth-1] + "…"
		}
		options[i] = huh.NewOption(label, s.Name)
	}

	height := rows - 2

	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select skills to install").
				Options(options...).
				Height(height).
				Value(&selected),
		),
	).WithHeight(height)

	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("prompt: %w", err)
	}

	// Filter selected
	selectedSet := make(map[string]bool, len(selected))
	for _, name := range selected {
		selectedSet[name] = true
	}

	var result []registry.DiscoveredSkill
	for _, s := range skills {
		if selectedSet[s.Name] {
			result = append(result, s)
		}
	}
	return result, nil
}

// Confirm presents a yes/no confirmation prompt.
// In non-interactive mode returns defaultVal.
func Confirm(message string, defaultVal bool) (bool, error) {
	if !IsInteractive() {
		return defaultVal, nil
	}

	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(message).
				Value(&confirmed).
				Affirmative("Yes").
				Negative("No"),
		),
	)

	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}
