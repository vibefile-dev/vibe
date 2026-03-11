package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProjectConfig represents the .vibe/config.yaml file — project-level configuration
// committed to version control. It never contains secrets.
type ProjectConfig struct {
	Servers      map[string]ServerConfig `yaml:"servers,omitempty"`
	SkillSources []string               `yaml:"skill_sources,omitempty"`
	Registry     *RegistryConfig        `yaml:"registry,omitempty"`
}

// ServerConfig defines how to reach an MCP server.
type ServerConfig struct {
	URL     string   `yaml:"url,omitempty"`
	Command string   `yaml:"command,omitempty"`
	Args    []string `yaml:"args,omitempty"`
}

// RegistryConfig defines a registry for dynamic server/skill resolution.
type RegistryConfig struct {
	URL string `yaml:"url"`
}

// LoadProjectConfig reads .vibe/config.yaml from the given repo root.
// Returns an empty config (not an error) if the file doesn't exist.
func LoadProjectConfig(repoRoot string) (*ProjectConfig, error) {
	path := filepath.Join(repoRoot, ".vibe", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProjectConfig{}, nil
		}
		return nil, fmt.Errorf("read .vibe/config.yaml: %w", err)
	}

	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse .vibe/config.yaml: %w", err)
	}
	return &cfg, nil
}

// SaveProjectConfig writes project config to .vibe/config.yaml, creating
// the directory if needed.
func SaveProjectConfig(repoRoot string, cfg *ProjectConfig) error {
	dir := filepath.Join(repoRoot, ".vibe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create .vibe directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
