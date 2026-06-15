package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SchemaVersion is the current config schema version.
const SchemaVersion = 2

// Hash kinds for SkillInfo.HashKind.
const (
	HashKindSHA1    = "sha1"     // local deterministic folder hash (clone/local sources)
	HashKindTreeSHA = "tree-sha" // GitHub git-tree SHA (fast-path, no clone)
)

// Install modes for SkillInfo.Mode.
const (
	ModeSymlink = "symlink"
	ModeCopy    = "copy"
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
	Description string    `json:"description,omitempty"`
	PathInRepo  string    `json:"path_in_repo"`
	FolderHash  string    `json:"folder_hash"`
	HashKind    string    `json:"hash_kind,omitempty"` // "sha1" | "tree-sha"
	Agents      []string  `json:"agents,omitempty"`    // agents this skill is linked into
	Mode        string    `json:"mode,omitempty"`      // "symlink" | "copy"
	Ref         string    `json:"ref,omitempty"`       // pinned branch/tag of the source
	Subpath     string    `json:"subpath,omitempty"`   // subpath within the source repo
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Project     string    `json:"project,omitempty"` // empty = global, otherwise absolute path to project root
}

// NewDefault creates a default config.
func NewDefault() *Config {
	return &Config{
		Version: SchemaVersion,
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
	if cfg.migrate() {
		// Best-effort sticky migration; ignore save errors (e.g. read-only FS).
		_ = cfg.Save()
	}
	return &cfg, nil
}

// LoadOrCreate loads the config, or returns a fresh default (without writing)
// if it does not exist yet. The file is only created on the first Save.
func LoadOrCreate() (*Config, error) {
	cfg, err := Load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewDefault(), nil
		}
		return nil, err
	}
	return cfg, nil
}

// migrate upgrades an older config in place. It returns true if anything changed.
func (c *Config) migrate() bool {
	if c.Version >= SchemaVersion {
		return false
	}

	if len(c.Agents) == 0 {
		c.Agents = []string{"claude-code"}
	}
	for name, skill := range c.Skills {
		if len(skill.Agents) == 0 {
			skill.Agents = append([]string(nil), c.Agents...)
		}
		if skill.Mode == "" {
			skill.Mode = ModeSymlink
		}
		if skill.HashKind == "" {
			skill.HashKind = HashKindSHA1
		}
		c.Skills[name] = skill
	}
	c.Version = SchemaVersion
	return true
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
