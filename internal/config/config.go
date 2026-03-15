package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Version int                  `json:"version"`
	Repos   map[string]RepoInfo  `json:"repos"`
	Skills  map[string]SkillInfo `json:"skills"`
	Agents  []string             `json:"agents"`
}

type RepoInfo struct {
	Alias   string    `json:"alias"`
	AddedAt time.Time `json:"added_at"`
}

type SkillInfo struct {
	Repo        string    `json:"repo"`
	PathInRepo  string    `json:"path_in_repo"`
	FolderHash  string    `json:"folder_hash"`
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Project     string    `json:"project,omitempty"` // empty = global, otherwise absolute path to project root
}

// NewDefault creates a default config.
func NewDefault() *Config {
	return &Config{
		Version: 1,
		Repos:   make(map[string]RepoInfo),
		Skills:  make(map[string]SkillInfo),
		Agents:  []string{"claude-code"},
	}
}

// Load reads the config from disk. Returns os.ErrNotExist if file is missing.
func Load() (*Config, error) {
	data, err := os.ReadFile(ConfigFile())
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Repos == nil {
		cfg.Repos = make(map[string]RepoInfo)
	}
	if cfg.Skills == nil {
		cfg.Skills = make(map[string]SkillInfo)
	}
	return &cfg, nil
}

// MustLoad loads the config or exits with a helpful message.
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "Config not found. Run 'skills init' first.")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

// Save writes the config to disk atomically.
func (c *Config) Save() error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')

	tmp := filepath.Join(dir, "config.tmp.json")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := os.Rename(tmp, ConfigFile()); err != nil {
		if removeErr := os.Remove(tmp); removeErr != nil {
			return fmt.Errorf("rename config: %w (also failed to remove temp file: %v)", err, removeErr)
		}
		return fmt.Errorf("rename config: %w", err)
	}
	return nil
}
