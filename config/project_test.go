package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProjectConfigMissing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadProjectConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config for missing file")
	}
	if len(cfg.Servers) != 0 {
		t.Error("expected empty servers for missing file")
	}
}

func TestLoadProjectConfigValid(t *testing.T) {
	dir := t.TempDir()
	vibeDir := filepath.Join(dir, ".vibe")
	os.MkdirAll(vibeDir, 0o755)

	content := `servers:
  fly-mcp:
    url: https://mcp.fly.io/sse
  postgres-mcp:
    command: npx @modelcontextprotocol/server-postgres
    args: ["--connection-string", "postgresql://localhost/mydb"]
skill_sources:
  - ./skills
  - ~/.vibe/skills
registry:
  url: https://aregistry.ai
`
	os.WriteFile(filepath.Join(vibeDir, "config.yaml"), []byte(content), 0o644)

	cfg, err := LoadProjectConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(cfg.Servers))
	}
	fly := cfg.Servers["fly-mcp"]
	if fly.URL != "https://mcp.fly.io/sse" {
		t.Errorf("expected fly-mcp URL, got %q", fly.URL)
	}
	pg := cfg.Servers["postgres-mcp"]
	if pg.Command != "npx @modelcontextprotocol/server-postgres" {
		t.Errorf("unexpected postgres command: %q", pg.Command)
	}
	if len(pg.Args) != 2 {
		t.Fatalf("expected 2 args for postgres, got %d", len(pg.Args))
	}

	if len(cfg.SkillSources) != 2 {
		t.Fatalf("expected 2 skill sources, got %d", len(cfg.SkillSources))
	}
	if cfg.SkillSources[0] != "./skills" {
		t.Errorf("expected first skill source ./skills, got %q", cfg.SkillSources[0])
	}

	if cfg.Registry == nil || cfg.Registry.URL != "https://aregistry.ai" {
		t.Error("expected registry URL")
	}
}

func TestSaveAndLoadProjectConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &ProjectConfig{
		Servers: map[string]ServerConfig{
			"test-mcp": {URL: "https://test.example.com/sse"},
		},
		SkillSources: []string{"./skills"},
	}

	if err := SaveProjectConfig(dir, cfg); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadProjectConfig(dir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(loaded.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(loaded.Servers))
	}
	if loaded.Servers["test-mcp"].URL != "https://test.example.com/sse" {
		t.Error("server URL mismatch after round-trip")
	}
}

func TestLoadProjectConfigInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	vibeDir := filepath.Join(dir, ".vibe")
	os.MkdirAll(vibeDir, 0o755)
	os.WriteFile(filepath.Join(vibeDir, "config.yaml"), []byte("{{invalid"), 0o644)

	_, err := LoadProjectConfig(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
