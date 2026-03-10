package llm

import (
	"strings"
	"testing"

	vibecontext "github.com/vibefile-dev/vibe/context"
	"github.com/vibefile-dev/vibe/parser"
)

func TestCleanScriptPlain(t *testing.T) {
	input := "#!/bin/bash\necho hello"
	if got := cleanScript(input); got != input {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestCleanScriptWithFences(t *testing.T) {
	input := "```bash\n#!/bin/bash\necho hello\n```"
	expected := "#!/bin/bash\necho hello"
	if got := cleanScript(input); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestCleanScriptWithFencesNoLang(t *testing.T) {
	input := "```\n#!/bin/bash\necho hello\n```"
	expected := "#!/bin/bash\necho hello"
	if got := cleanScript(input); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestCleanScriptTrimsWhitespace(t *testing.T) {
	input := "  \n#!/bin/bash\necho hello\n  "
	got := cleanScript(input)
	if strings.HasPrefix(got, " ") || strings.HasSuffix(got, " ") {
		t.Errorf("expected trimmed, got %q", got)
	}
}

func TestIsAnthropicModel(t *testing.T) {
	tests := map[string]bool{
		"claude-sonnet-4-6": true,
		"claude-haiku-4-5":  true,
		"claude-opus-4-6":   true,
		"gpt-4o":            false,
		"o1-mini":           false,
		"random-model":      false,
	}
	for model, want := range tests {
		if got := isAnthropicModel(model); got != want {
			t.Errorf("isAnthropicModel(%q) = %v, want %v", model, got, want)
		}
	}
}

func TestIsOpenAIModel(t *testing.T) {
	tests := map[string]bool{
		"gpt-4o":            true,
		"gpt-4-turbo":       true,
		"o1-mini":           true,
		"o3-mini":           true,
		"claude-sonnet-4-6": false,
		"random-model":      false,
	}
	for model, want := range tests {
		if got := isOpenAIModel(model); got != want {
			t.Errorf("isOpenAIModel(%q) = %v, want %v", model, got, want)
		}
	}
}

func TestSystemPromptNotEmpty(t *testing.T) {
	sp := SystemPrompt()
	if sp == "" {
		t.Error("system prompt should not be empty")
	}
	if !strings.Contains(sp, "shell script") {
		t.Error("system prompt should mention shell script")
	}
}

func TestBuildPrompt(t *testing.T) {
	target := &parser.Target{Name: "build", Recipe: "compile $(env)"}
	ctx := &vibecontext.Collected{
		FileTree:     "main.go\n",
		ProjectFiles: make(map[string]string),
	}
	vars := map[string]string{"env": "prod"}

	prompt := BuildPrompt(target, ctx, vars)
	if !strings.Contains(prompt, "compile prod") {
		t.Error("expected substituted recipe in prompt")
	}
	if !strings.Contains(prompt, "main.go") {
		t.Error("expected file tree in prompt")
	}
}

func TestBuildRetryPrompt(t *testing.T) {
	target := &parser.Target{Name: "build", Recipe: "compile"}
	ctx := &vibecontext.Collected{
		FileTree:     ".\n",
		ProjectFiles: make(map[string]string),
	}

	prompt := BuildRetryPrompt(target, ctx, nil, "#!/bin/bash\nexit 1", "command not found: go")
	if !strings.Contains(prompt, "previous script") {
		t.Error("expected retry context in prompt")
	}
	if !strings.Contains(prompt, "command not found") {
		t.Error("expected error output in prompt")
	}
}

func TestBuildRetryPromptTruncatesLongOutput(t *testing.T) {
	target := &parser.Target{Name: "build", Recipe: "compile"}
	ctx := &vibecontext.Collected{
		FileTree:     ".\n",
		ProjectFiles: make(map[string]string),
	}
	longErr := strings.Repeat("x", 5000)

	prompt := BuildRetryPrompt(target, ctx, nil, "script", longErr)
	if !strings.Contains(prompt, "truncated") {
		t.Error("expected truncation notice for long error output")
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("key", "claude-sonnet-4-6")
	if c.APIKey != "key" {
		t.Errorf("expected key, got %q", c.APIKey)
	}
	if c.Model != "claude-sonnet-4-6" {
		t.Errorf("expected claude-sonnet-4-6, got %q", c.Model)
	}
	if c.HTTPClient == nil {
		t.Error("expected non-nil HTTP client")
	}
}

func TestGenerateUnsupportedModel(t *testing.T) {
	c := NewClient("key", "unknown-model")
	_, err := c.Generate("sys", "user")
	if err == nil {
		t.Fatal("expected error for unsupported model")
	}
	if !strings.Contains(err.Error(), "unsupported model") {
		t.Errorf("unexpected error: %v", err)
	}
}
