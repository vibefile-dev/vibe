package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveFromRepoSkillsDir(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "go-test")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Run go test with race detection"), 0o644)

	info, err := Resolve(dir, "go-test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Content != "Run go test with race detection" {
		t.Errorf("unexpected content: %q", info.Content)
	}
	if info.Name != "go-test" {
		t.Errorf("unexpected name: %q", info.Name)
	}
	if info.Hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestResolveFromVibeSkillsDir(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, ".vibe", "skills", "deploy-fly")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Deploy to Fly.io"), 0o644)

	info, err := Resolve(dir, "deploy-fly", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Content != "Deploy to Fly.io" {
		t.Errorf("unexpected content: %q", info.Content)
	}
}

func TestResolveFromExtraSources(t *testing.T) {
	dir := t.TempDir()
	extraDir := filepath.Join(dir, "custom-skills")
	skillDir := filepath.Join(extraDir, "my-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Custom skill instructions"), 0o644)

	info, err := Resolve(dir, "my-skill", []string{extraDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Content != "Custom skill instructions" {
		t.Errorf("unexpected content: %q", info.Content)
	}
}

func TestResolveNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Resolve(dir, "nonexistent-skill", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent skill")
	}
	if !strings.Contains(err.Error(), "nonexistent-skill") {
		t.Errorf("error should mention skill name: %v", err)
	}
}

func TestResolvePriority(t *testing.T) {
	dir := t.TempDir()

	repoSkill := filepath.Join(dir, "skills", "test-skill")
	os.MkdirAll(repoSkill, 0o755)
	os.WriteFile(filepath.Join(repoSkill, "SKILL.md"), []byte("repo version"), 0o644)

	vibeSkill := filepath.Join(dir, ".vibe", "skills", "test-skill")
	os.MkdirAll(vibeSkill, 0o755)
	os.WriteFile(filepath.Join(vibeSkill, "SKILL.md"), []byte("vibe version"), 0o644)

	info, err := Resolve(dir, "test-skill", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Content != "repo version" {
		t.Errorf("expected repo version to win, got %q", info.Content)
	}
}

func TestResolveWithFrontmatter(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "go-test")
	os.MkdirAll(skillDir, 0o755)

	content := "---\ndescription: Run Go tests with race detection\n---\n\n# Go Test\n\nRun `go test -race ./...`"
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644)

	info, err := Resolve(dir, "go-test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Description != "Run Go tests with race detection" {
		t.Errorf("unexpected description: %q", info.Description)
	}
	if info.Content != "# Go Test\n\nRun `go test -race ./...`" {
		t.Errorf("unexpected content: %q", info.Content)
	}
	if info.RawContent != content {
		t.Errorf("expected raw content to include frontmatter")
	}
}

func TestResolveWithoutFrontmatter(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "simple")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Just instructions"), 0o644)

	info, err := Resolve(dir, "simple", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Description != "" {
		t.Errorf("expected empty description, got %q", info.Description)
	}
	if info.Content != "Just instructions" {
		t.Errorf("unexpected content: %q", info.Content)
	}
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantDesc string
		wantBody string
	}{
		{
			name:     "with frontmatter",
			input:    "---\ndescription: Test skill\n---\n\nBody here",
			wantDesc: "Test skill",
			wantBody: "Body here",
		},
		{
			name:     "without frontmatter",
			input:    "Just plain markdown",
			wantDesc: "",
			wantBody: "Just plain markdown",
		},
		{
			name:     "unclosed frontmatter",
			input:    "---\ndescription: Test\nBody without closing",
			wantDesc: "",
			wantBody: "---\ndescription: Test\nBody without closing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, body := parseFrontmatter(tt.input)
			if desc != tt.wantDesc {
				t.Errorf("description = %q, want %q", desc, tt.wantDesc)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := expandPath("~/foo/bar")
	expected := filepath.Join(home, "foo", "bar")
	if got != expected {
		t.Errorf("expandPath(~/foo/bar) = %q, want %q", got, expected)
	}

	got2 := expandPath("/absolute/path")
	if got2 != "/absolute/path" {
		t.Errorf("expandPath(/absolute/path) = %q, want /absolute/path", got2)
	}
}
