package require

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/vibefile-dev/vibe/parser"
)

// CheckResult describes the outcome of a single @require evaluation.
type CheckResult struct {
	Condition string
	Passed    bool
	Message   string
}

// Evaluate runs all @require directives on a target and returns any that failed.
// Returns nil if all requirements are met or the target has no @require directives.
func Evaluate(repoRoot string, target *parser.Target) []CheckResult {
	var failed []CheckResult
	for _, d := range target.Directives {
		if d.Name != "require" {
			continue
		}
		r := check(repoRoot, d.Args)
		if !r.Passed {
			failed = append(failed, r)
		}
	}
	return failed
}

func check(repoRoot, condition string) CheckResult {
	lower := strings.ToLower(strings.TrimSpace(condition))

	switch {
	case matchesCleanGit(lower):
		return checkCleanGit(repoRoot, condition)
	case matchesBranch(lower):
		return checkBranch(repoRoot, condition, lower)
	case matchesDeferrable(lower):
		return CheckResult{Condition: condition, Passed: true, Message: "deferred to dependency chain"}
	default:
		return checkShellCondition(repoRoot, condition)
	}
}

func matchesCleanGit(s string) bool {
	return (strings.Contains(s, "clean") &&
		(strings.Contains(s, "git") || strings.Contains(s, "working"))) ||
		s == "no uncommitted changes"
}

func matchesBranch(s string) bool {
	return strings.HasPrefix(s, "on branch ") || strings.HasPrefix(s, "branch ")
}

func matchesDeferrable(s string) bool {
	return strings.Contains(s, "passing tests") || strings.Contains(s, "tests pass")
}

func checkCleanGit(repoRoot, condition string) CheckResult {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return CheckResult{
			Condition: condition,
			Passed:    false,
			Message:   fmt.Sprintf("failed to check git status: %v", err),
		}
	}
	if strings.TrimSpace(string(out)) != "" {
		return CheckResult{
			Condition: condition,
			Passed:    false,
			Message:   "working directory has uncommitted changes",
		}
	}
	return CheckResult{Condition: condition, Passed: true, Message: "working directory is clean"}
}

func checkBranch(repoRoot, condition, lower string) CheckResult {
	// Extract expected branch name from "on branch main" or "branch main"
	expected := strings.TrimPrefix(lower, "on branch ")
	expected = strings.TrimPrefix(expected, "branch ")
	expected = strings.TrimSpace(expected)

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return CheckResult{
			Condition: condition,
			Passed:    false,
			Message:   fmt.Sprintf("failed to get current branch: %v", err),
		}
	}
	actual := strings.TrimSpace(string(out))
	if actual != expected {
		return CheckResult{
			Condition: condition,
			Passed:    false,
			Message:   fmt.Sprintf("expected branch %q, currently on %q", expected, actual),
		}
	}
	return CheckResult{Condition: condition, Passed: true, Message: fmt.Sprintf("on branch %s", actual)}
}

// checkShellCondition tries to evaluate an unrecognized condition by running
// "command -v <tool>" if the condition looks like a tool name, otherwise passes.
func checkShellCondition(repoRoot, condition string) CheckResult {
	lower := strings.ToLower(condition)

	// "command X installed" or "X is installed" → check command exists
	if strings.Contains(lower, "installed") {
		words := strings.Fields(condition)
		for _, w := range words {
			w = strings.TrimSuffix(strings.ToLower(w), ",")
			if w == "installed" || w == "is" || w == "be" || w == "must" || w == "should" || w == "command" {
				continue
			}
			cmd := exec.Command("command", "-v", w)
			cmd.Dir = repoRoot
			if err := cmd.Run(); err != nil {
				return CheckResult{
					Condition: condition,
					Passed:    false,
					Message:   fmt.Sprintf("%s not found in PATH", w),
				}
			}
			return CheckResult{Condition: condition, Passed: true, Message: fmt.Sprintf("%s found", w)}
		}
	}

	// Unrecognized — pass with a note
	return CheckResult{
		Condition: condition,
		Passed:    true,
		Message:   "unrecognized condition (skipped)",
	}
}
