package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// VibeConfig represents the ~/.vibeconfig file.
type VibeConfig struct {
	DefaultModel string            `yaml:"default_model"`
	AnthropicKey string            `yaml:"anthropic_key"`
	OpenAIKey    string            `yaml:"openai_key"`
	ServerTokens map[string]string `yaml:"server_tokens,omitempty"`
}

// ResolveAPIKey resolves the API key using the priority chain:
// 1. --api-key CLI flag
// 2. VIBE_API_KEY env var
// 3. Provider-specific env var inferred from model
// 4. ~/.vibeconfig
func ResolveAPIKey(flagValue, model string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}

	if key := os.Getenv("VIBE_API_KEY"); key != "" {
		return key, nil
	}

	providerVar := providerEnvVar(model)
	if providerVar != "" {
		if key := os.Getenv(providerVar); key != "" {
			return key, nil
		}
	}

	cfg, err := loadVibeConfig()
	if err == nil {
		if key := cfg.keyForModel(model); key != "" {
			return key, nil
		}
	}

	hint := ""
	if providerVar != "" {
		hint = fmt.Sprintf(" (tried %s)", providerVar)
	}
	return "", fmt.Errorf("no API key found for model %q%s — set VIBE_API_KEY or add it to ~/.vibeconfig", model, hint)
}

// ResolveModel returns the effective model, applying defaults.
// Priority: target override → Vibefile global → ~/.vibeconfig default → hardcoded default.
func ResolveModel(targetModel, vibefileModel string) string {
	if targetModel != "" {
		return targetModel
	}
	if vibefileModel != "" {
		return vibefileModel
	}
	cfg, err := loadVibeConfig()
	if err == nil && cfg.DefaultModel != "" {
		return cfg.DefaultModel
	}
	return "claude-sonnet-4-6"
}

func providerEnvVar(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "claude") || strings.Contains(m, "haiku") || strings.Contains(m, "opus") || strings.Contains(m, "sonnet"):
		return "ANTHROPIC_API_KEY"
	case strings.Contains(m, "gpt") || strings.Contains(m, "o1") || strings.Contains(m, "o3"):
		return "OPENAI_API_KEY"
	default:
		return ""
	}
}

func (c *VibeConfig) keyForModel(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "claude") || strings.Contains(m, "haiku") || strings.Contains(m, "opus") || strings.Contains(m, "sonnet"):
		return c.AnthropicKey
	case strings.Contains(m, "gpt") || strings.Contains(m, "o1") || strings.Contains(m, "o3"):
		return c.OpenAIKey
	default:
		return ""
	}
}

// LoadVibeConfig reads and parses ~/.vibeconfig. Returns nil and an error
// if the file doesn't exist or can't be parsed.
func LoadVibeConfig() (*VibeConfig, error) {
	return loadVibeConfig()
}

func loadVibeConfig() (*VibeConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(home, ".vibeconfig"))
	if err != nil {
		return nil, err
	}
	var cfg VibeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
