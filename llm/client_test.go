package llm

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"

	vibecontext "github.com/vibefile-dev/vibe/context"
	"github.com/vibefile-dev/vibe/parser"
	"github.com/vibefile-dev/vibe/skill"
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

func TestBuildPromptNoSkillInjection(t *testing.T) {
	target := &parser.Target{Name: "test", Recipe: "run tests"}
	ctx := &vibecontext.Collected{
		FileTree:     "main.go\n",
		ProjectFiles: make(map[string]string),
	}

	prompt := BuildPrompt(target, ctx, nil)
	if strings.Contains(prompt, "Skill instructions") {
		t.Error("prompt should not contain skill instructions — skills are provided via tools")
	}
	if !strings.Contains(prompt, "run tests") {
		t.Error("expected recipe in prompt")
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
	_, err := c.Generate("sys", "user", nil)
	if err == nil {
		t.Fatal("expected error for unsupported model")
	}
	if !strings.Contains(err.Error(), "unsupported model") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerateSkillsRequireAnthropic(t *testing.T) {
	c := NewClient("key", "gpt-4o")
	skills := []*skill.SkillInfo{
		{Name: "test-skill", Description: "A test", Content: "instructions"},
	}
	_, err := c.Generate("sys", "user", skills)
	if err == nil {
		t.Fatal("expected error when using skills with OpenAI model")
	}
	if !strings.Contains(err.Error(), "Anthropic model") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildSkillToolDescription(t *testing.T) {
	skills := []*skill.SkillInfo{
		{Name: "go-test", Description: "Run Go tests with race detection"},
		{Name: "deploy", Description: "Deploy to production"},
	}

	desc := buildSkillToolDescription(skills)
	if !strings.Contains(desc, "<available_skills>") {
		t.Error("expected available_skills XML block")
	}
	if !strings.Contains(desc, "<name>go-test</name>") {
		t.Error("expected go-test skill name")
	}
	if !strings.Contains(desc, "race detection") {
		t.Error("expected go-test description")
	}
	if !strings.Contains(desc, "<name>deploy</name>") {
		t.Error("expected deploy skill name")
	}
}

func TestBuildSkillToolDescriptionNoDescription(t *testing.T) {
	skills := []*skill.SkillInfo{
		{Name: "my-skill", Description: ""},
	}

	desc := buildSkillToolDescription(skills)
	if !strings.Contains(desc, "Skill: my-skill") {
		t.Error("expected fallback description when description is empty")
	}
}

func TestExtractText(t *testing.T) {
	// extractText is tested indirectly through callAnthropic, but we test
	// the cleanScript helper directly since extractText depends on SDK types
	script := "```bash\n#!/bin/bash\necho hello\n```"
	cleaned := cleanScript(script)
	if cleaned != "#!/bin/bash\necho hello" {
		t.Errorf("unexpected cleaned script: %q", cleaned)
	}
}

func TestHandleSkillToolCallFound(t *testing.T) {
	skills := []*skill.SkillInfo{
		{Name: "go-test", RawContent: "---\ndescription: Run tests\n---\n# Go Test"},
		{Name: "deploy", RawContent: "deploy instructions"},
	}
	toolUse := anthropic.ToolUseBlock{
		ID:    "tool_123",
		Name:  "Skill",
		Input: json.RawMessage(`{"command":"go-test"}`),
	}

	name, content, err := handleSkillToolCall(toolUse, skills)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "go-test" {
		t.Errorf("expected skill name go-test, got %q", name)
	}
	if content != skills[0].RawContent {
		t.Errorf("expected raw content, got %q", content)
	}
}

func TestHandleSkillToolCallNotFound(t *testing.T) {
	skills := []*skill.SkillInfo{
		{Name: "go-test", RawContent: "content"},
	}
	toolUse := anthropic.ToolUseBlock{
		ID:    "tool_456",
		Name:  "Skill",
		Input: json.RawMessage(`{"command":"nonexistent"}`),
	}

	name, _, err := handleSkillToolCall(toolUse, skills)
	if err == nil {
		t.Fatal("expected error for nonexistent skill")
	}
	if name != "nonexistent" {
		t.Errorf("expected skill name in error return, got %q", name)
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention skill name: %v", err)
	}
}

func TestHandleSkillToolCallInvalidJSON(t *testing.T) {
	toolUse := anthropic.ToolUseBlock{
		ID:    "tool_789",
		Name:  "Skill",
		Input: json.RawMessage(`{invalid`),
	}

	_, _, err := handleSkillToolCall(toolUse, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid skill tool input") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestOnSkillInvokedCallback(t *testing.T) {
	client := NewClient("key", "claude-sonnet-4-6")
	var events []SkillEvent
	client.OnSkillInvoked = func(evt SkillEvent) {
		events = append(events, evt)
	}

	if client.OnSkillInvoked == nil {
		t.Fatal("expected OnSkillInvoked to be set")
	}
	client.OnSkillInvoked(SkillEvent{SkillName: "test", Iteration: 1})
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].SkillName != "test" || events[0].Iteration != 1 {
		t.Errorf("unexpected event: %+v", events[0])
	}
}
