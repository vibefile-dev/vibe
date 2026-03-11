package require

import (
	"os"
	"testing"

	"github.com/vibefile-dev/vibe/parser"
)

func TestEvaluateNoRequires(t *testing.T) {
	target := &parser.Target{
		Name:   "build",
		Recipe: "compile",
	}
	failed := Evaluate(".", target)
	if len(failed) != 0 {
		t.Errorf("expected no failures for target without @require, got %d", len(failed))
	}
}

func TestEvaluateCleanGitPatterns(t *testing.T) {
	patterns := []string{
		"clean git status",
		"clean working directory",
		"no uncommitted changes",
	}
	for _, p := range patterns {
		if !matchesCleanGit(p) {
			t.Errorf("expected %q to match clean git pattern", p)
		}
	}
}

func TestEvaluateBranchPatterns(t *testing.T) {
	if !matchesBranch("on branch main") {
		t.Error("expected 'on branch main' to match branch pattern")
	}
	if !matchesBranch("branch release") {
		t.Error("expected 'branch release' to match branch pattern")
	}
	if matchesBranch("deploy to branch") {
		t.Error("'deploy to branch' should not match branch pattern")
	}
}

func TestEvaluateDeferrablePatterns(t *testing.T) {
	if !matchesDeferrable("passing tests") {
		t.Error("expected 'passing tests' to be deferrable")
	}
	if !matchesDeferrable("all tests pass") {
		t.Error("expected 'all tests pass' to be deferrable")
	}
}

func TestCheckCleanGit(t *testing.T) {
	// This test works in the current repo — which may or may not be clean.
	// We just verify it doesn't error out.
	cwd, _ := os.Getwd()
	r := checkCleanGit(cwd, "clean git status")
	if r.Condition != "clean git status" {
		t.Errorf("expected condition to be preserved, got %q", r.Condition)
	}
	if r.Message == "" {
		t.Error("expected a message")
	}
}

func TestEvaluateWithFailingRequire(t *testing.T) {
	target := &parser.Target{
		Name:   "deploy",
		Recipe: "deploy to prod",
		Directives: []parser.Directive{
			{Name: "require", Args: "on branch nonexistent-branch-xyz"},
		},
	}
	cwd, _ := os.Getwd()
	failed := Evaluate(cwd, target)
	if len(failed) == 0 {
		t.Error("expected failure for wrong branch requirement")
	}
	if failed[0].Condition != "on branch nonexistent-branch-xyz" {
		t.Errorf("unexpected condition: %q", failed[0].Condition)
	}
}

func TestEvaluateMultipleRequires(t *testing.T) {
	target := &parser.Target{
		Name:   "deploy",
		Recipe: "deploy",
		Directives: []parser.Directive{
			{Name: "require", Args: "on branch nonexistent-xyz"},
			{Name: "skill", Args: "go-test"},
		},
	}
	cwd, _ := os.Getwd()
	failed := Evaluate(cwd, target)
	if len(failed) != 1 {
		t.Errorf("expected 1 failure (only @require, not @skill), got %d", len(failed))
	}
}

func TestCheckShellConditionUnrecognized(t *testing.T) {
	r := checkShellCondition(".", "the sky is blue")
	if !r.Passed {
		t.Error("unrecognized conditions should pass with a note")
	}
	if r.Message != "unrecognized condition (skipped)" {
		t.Errorf("unexpected message: %q", r.Message)
	}
}
