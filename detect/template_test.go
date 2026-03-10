package detect

import (
	"strings"
	"testing"
)

func TestGenerateEmptyTemplate(t *testing.T) {
	tmpl := &Template{}
	out := Generate(tmpl)
	if out != "\n" {
		t.Errorf("expected single newline for empty template, got %q", out)
	}
}

func TestGenerateVariables(t *testing.T) {
	tmpl := &Template{
		Variables: []TemplateVariable{
			{Key: "model", Value: "claude-sonnet-4-6"},
			{Key: "env", Value: "production"},
		},
	}
	out := Generate(tmpl)
	if !strings.Contains(out, "model") || !strings.Contains(out, "claude-sonnet-4-6") {
		t.Error("expected model variable in output")
	}
	if !strings.Contains(out, "env") || !strings.Contains(out, "production") {
		t.Error("expected env variable in output")
	}
}

func TestGenerateTargetNoDeps(t *testing.T) {
	tmpl := &Template{
		Targets: []TemplateTarget{
			{Name: "build", Recipe: "compile the project"},
		},
	}
	out := Generate(tmpl)
	if !strings.Contains(out, "build:") {
		t.Error("expected target header")
	}
	if !strings.Contains(out, `"compile the project"`) {
		t.Error("expected recipe in output")
	}
}

func TestGenerateTargetWithDeps(t *testing.T) {
	tmpl := &Template{
		Targets: []TemplateTarget{
			{Name: "deploy", Dependencies: []string{"test", "build"}, Recipe: "ship it"},
		},
	}
	out := Generate(tmpl)
	if !strings.Contains(out, "deploy: test build:") {
		t.Errorf("expected dependency syntax, got:\n%s", out)
	}
}

func TestGenerateTargetWithDirectives(t *testing.T) {
	tmpl := &Template{
		Targets: []TemplateTarget{
			{Name: "deploy", Recipe: "ship it", Directives: []string{"@require clean git status"}},
		},
	}
	out := Generate(tmpl)
	if !strings.Contains(out, "@require clean git status") {
		t.Error("expected directive in output")
	}
}

func TestGenerateSections(t *testing.T) {
	tmpl := &Template{
		Targets: []TemplateTarget{
			{Name: "build", Recipe: "compile", Section: "Development"},
			{Name: "deploy", Recipe: "ship it", Section: "Operations"},
		},
	}
	out := Generate(tmpl)
	if !strings.Contains(out, "Development") {
		t.Error("expected Development section header")
	}
	if !strings.Contains(out, "Operations") {
		t.Error("expected Operations section header")
	}
}

func TestMergeAddonsAppendsTargets(t *testing.T) {
	tmpl := &Template{
		Targets: []TemplateTarget{
			{Name: "build", Recipe: "compile"},
		},
	}
	addon := &AddonResult{
		Label: "Docker",
		Targets: []TemplateTarget{
			{Name: "docker-build", Recipe: "build docker image"},
		},
	}
	MergeAddons(tmpl, []*AddonResult{addon})
	if len(tmpl.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(tmpl.Targets))
	}
	if tmpl.Targets[1].Name != "docker-build" {
		t.Errorf("expected docker-build, got %q", tmpl.Targets[1].Name)
	}
}

func TestMergeAddonsSkipsDuplicates(t *testing.T) {
	tmpl := &Template{
		Targets: []TemplateTarget{
			{Name: "build", Recipe: "compile"},
		},
	}
	addon := &AddonResult{
		Label: "Conflicting",
		Targets: []TemplateTarget{
			{Name: "build", Recipe: "different recipe"},
		},
	}
	MergeAddons(tmpl, []*AddonResult{addon})
	if len(tmpl.Targets) != 1 {
		t.Fatalf("expected 1 target (dup skipped), got %d", len(tmpl.Targets))
	}
	if tmpl.Targets[0].Recipe != "compile" {
		t.Error("expected original recipe to be preserved")
	}
}

func TestPrefixAddonResult(t *testing.T) {
	r := &AddonResult{
		Label: "Docker",
		Targets: []TemplateTarget{
			{Name: "build", Recipe: "build image", Dependencies: []string{"test"}},
		},
	}
	PrefixAddonResult(r, "api")
	if r.Label != "api/Docker" {
		t.Errorf("expected prefixed label, got %q", r.Label)
	}
	if r.Targets[0].Name != "api-build" {
		t.Errorf("expected prefixed name, got %q", r.Targets[0].Name)
	}
	if r.Targets[0].Dependencies[0] != "api-test" {
		t.Errorf("expected prefixed dep, got %q", r.Targets[0].Dependencies[0])
	}
	if !strings.HasPrefix(r.Targets[0].Recipe, "in the api/ directory") {
		t.Errorf("expected prefixed recipe, got %q", r.Targets[0].Recipe)
	}
}

func TestSectionPadding(t *testing.T) {
	pad := sectionPadding("Dev")
	if !strings.HasPrefix(pad, "─") {
		t.Error("expected padding to be dashes")
	}
	if len(pad) < 2 {
		t.Error("expected non-trivial padding")
	}
}

func TestSectionPaddingLongName(t *testing.T) {
	pad := sectionPadding(strings.Repeat("x", 100))
	if pad != "──" {
		t.Errorf("expected minimal padding for long section, got %q", pad)
	}
}
