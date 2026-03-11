package parser

import (
	"testing"
)

func TestParseSimpleVibefile(t *testing.T) {
	input := `
model = claude-sonnet-4-6
env = production

build:
    "compile and bundle the project for $(env)"

test:
    "run the test suite"

deploy: test build:
    "deploy to $(env) on fly.io"
    @require clean git status
    @mcp fly-mcp
`
	vf, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(vf.Variables) != 2 {
		t.Errorf("expected 2 variables, got %d", len(vf.Variables))
	}
	if vf.Variables["model"] != "claude-sonnet-4-6" {
		t.Errorf("expected model = claude-sonnet-4-6, got %q", vf.Variables["model"])
	}
	if vf.Variables["env"] != "production" {
		t.Errorf("expected env = production, got %q", vf.Variables["env"])
	}

	if len(vf.Targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(vf.Targets))
	}

	build := vf.Targets["build"]
	if build.Recipe != "compile and bundle the project for $(env)" {
		t.Errorf("unexpected build recipe: %q", build.Recipe)
	}
	if build.ExecutionMode() != "codegen" {
		t.Errorf("expected codegen mode for build, got %q", build.ExecutionMode())
	}

	deploy := vf.Targets["deploy"]
	if len(deploy.Dependencies) != 2 {
		t.Errorf("expected 2 deps for deploy, got %d", len(deploy.Dependencies))
	}
	if deploy.ExecutionMode() != "agent" {
		t.Errorf("expected agent mode for deploy, got %q", deploy.ExecutionMode())
	}
	if !deploy.HasDirective("require") {
		t.Error("expected deploy to have @require directive")
	}
	if !deploy.HasDirective("mcp") {
		t.Error("expected deploy to have @mcp directive")
	}
}

func TestParseDuplicateTarget(t *testing.T) {
	input := `
build:
    "compile"

build:
    "compile again"
`
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for duplicate target")
	}
}

func TestParseUnknownDependency(t *testing.T) {
	input := `
deploy: nonexistent:
    "deploy"
`
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for unknown dependency")
	}
}

func TestParseNoRecipe(t *testing.T) {
	input := `
build:
    @require clean git status
`
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for target with no recipe and no @skill")
	}
}

func TestParseSkillWithoutRecipe(t *testing.T) {
	input := `
test:
    @skill python-test
`
	vf, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	test := vf.Targets["test"]
	if test.ExecutionMode() != "skill" {
		t.Errorf("expected skill mode, got %q", test.ExecutionMode())
	}
}

func TestParseSkillWithRecipe(t *testing.T) {
	input := `
test:
    @skill go-test
    "run the Go tests for this project"
`
	vf, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	test := vf.Targets["test"]
	if !test.HasDirective("skill") {
		t.Error("expected test target to have @skill directive")
	}
	if test.DirectiveArgs("skill") != "go-test" {
		t.Errorf("expected skill arg go-test, got %q", test.DirectiveArgs("skill"))
	}
	if test.Recipe != "run the Go tests for this project" {
		t.Errorf("unexpected recipe: %q", test.Recipe)
	}
	if test.ExecutionMode() != "skill" {
		t.Errorf("expected skill mode, got %q", test.ExecutionMode())
	}
}

func TestSubstituteVars(t *testing.T) {
	vars := map[string]string{"env": "production", "project": "myapp"}
	result := SubstituteVars("deploy $(project) to $(env)", vars)
	expected := "deploy myapp to production"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestParseTargetModelOverride(t *testing.T) {
	input := `
model = claude-haiku-4-5

release:
    model = claude-opus-4-6
    "bump version and release"
`
	vf, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if vf.Variables["model"] != "claude-haiku-4-5" {
		t.Errorf("expected global model claude-haiku-4-5, got %q", vf.Variables["model"])
	}
	release := vf.Targets["release"]
	if release.Model != "claude-opus-4-6" {
		t.Errorf("expected target model claude-opus-4-6, got %q", release.Model)
	}
}

func TestParseMultiLineRecipe(t *testing.T) {
	input := `
test:
    "run tests"

build:
    "compile"

release: test build:
    "bump the version, update the changelog,
     tag the commit, and push to origin"
`
	vf, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	release := vf.Targets["release"]
	expected := "bump the version, update the changelog, tag the commit, and push to origin"
	if release.Recipe != expected {
		t.Errorf("expected recipe %q, got %q", expected, release.Recipe)
	}
}

func TestParseMultiLineRecipeThreeLines(t *testing.T) {
	input := `
deploy:
    "compile the Go binary named vibe from the module root.
     use -trimpath and -ldflags to strip debug info
     for a smaller binary."
`
	vf, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	deploy := vf.Targets["deploy"]
	if deploy.Recipe == "" {
		t.Fatal("expected non-empty recipe")
	}
	if !contains(deploy.Recipe, "compile the Go binary") || !contains(deploy.Recipe, "smaller binary") {
		t.Errorf("recipe doesn't contain expected content: %q", deploy.Recipe)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}
func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestParseUnterminatedRecipe(t *testing.T) {
	input := `
build:
    "this recipe never closes
`
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for unterminated recipe")
	}
}

func TestParseDeclarationOrder(t *testing.T) {
	input := `
charlie:
    "third"

alpha:
    "first"

bravo:
    "second"
`
	vf, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	expected := []string{"charlie", "alpha", "bravo"}
	if len(vf.Order) != len(expected) {
		t.Fatalf("expected %d targets in order, got %d", len(expected), len(vf.Order))
	}
	for i, name := range expected {
		if vf.Order[i] != name {
			t.Errorf("order[%d]: expected %q, got %q", i, name, vf.Order[i])
		}
	}
}
