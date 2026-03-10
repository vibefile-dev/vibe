package config

import (
	"testing"
)

func TestProviderEnvVar(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"claude-sonnet-4-6", "ANTHROPIC_API_KEY"},
		{"claude-haiku-4-5", "ANTHROPIC_API_KEY"},
		{"claude-opus-4-6", "ANTHROPIC_API_KEY"},
		{"gpt-4o", "OPENAI_API_KEY"},
		{"o1-mini", "OPENAI_API_KEY"},
		{"o3-mini", "OPENAI_API_KEY"},
		{"some-unknown-model", ""},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := providerEnvVar(tt.model)
			if got != tt.expected {
				t.Errorf("providerEnvVar(%q) = %q, want %q", tt.model, got, tt.expected)
			}
		})
	}
}

func TestResolveModelPriority(t *testing.T) {
	if got := ResolveModel("target-model", "vibefile-model"); got != "target-model" {
		t.Errorf("expected target override, got %q", got)
	}
	if got := ResolveModel("", "vibefile-model"); got != "vibefile-model" {
		t.Errorf("expected vibefile model, got %q", got)
	}
	got := ResolveModel("", "")
	if got == "" {
		t.Error("expected a default model, got empty string")
	}
}

func TestResolveAPIKeyFlag(t *testing.T) {
	key, err := ResolveAPIKey("flag-key", "claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "flag-key" {
		t.Errorf("expected flag-key, got %q", key)
	}
}

func TestResolveAPIKeyEnvVar(t *testing.T) {
	t.Setenv("VIBE_API_KEY", "env-key")
	key, err := ResolveAPIKey("", "claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "env-key" {
		t.Errorf("expected env-key, got %q", key)
	}
}

func TestResolveAPIKeyProviderEnvVar(t *testing.T) {
	t.Setenv("VIBE_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-key")
	key, err := ResolveAPIKey("", "claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "anthropic-key" {
		t.Errorf("expected anthropic-key, got %q", key)
	}
}

func TestResolveAPIKeyNoKeyFound(t *testing.T) {
	t.Setenv("VIBE_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	_, err := ResolveAPIKey("", "claude-sonnet-4-6")
	if err == nil {
		t.Fatal("expected error when no key is found")
	}
}

func TestKeyForModel(t *testing.T) {
	cfg := &VibeConfig{
		AnthropicKey: "ak",
		OpenAIKey:    "ok",
	}
	if got := cfg.keyForModel("claude-sonnet-4-6"); got != "ak" {
		t.Errorf("expected anthropic key, got %q", got)
	}
	if got := cfg.keyForModel("gpt-4o"); got != "ok" {
		t.Errorf("expected openai key, got %q", got)
	}
	if got := cfg.keyForModel("unknown"); got != "" {
		t.Errorf("expected empty for unknown model, got %q", got)
	}
}
