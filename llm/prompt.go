package llm

import (
	"fmt"
	"strings"

	vibecontext "github.com/vibefiledev/vibe/context"
	"github.com/vibefiledev/vibe/parser"
)

const systemPrompt = `You are a task runner. Your job is to generate a shell script that accomplishes the described task.

Script structure:
1. Start with #!/bin/bash and set -euo pipefail
2. Preflight section: verify that the tools the script ACTUALLY CALLS are available.
   - Use "command -v <tool>" to check availability.
   - Only add version checks when the task is version-sensitive (build, test, compile, deploy).
     Do NOT add version checks for simple tasks like formatting, cleaning, or listing.
   - If a preflight check fails, print a clear error ("error: <tool> is required but not installed") and exit 2.
   - NEVER install system-level dependencies. Only check that they exist.
   - Keep preflight minimal — check only what the script itself needs.
3. Task section: perform the actual work.

Exit code convention (you MUST follow this):
- exit 0: success
- exit 1: the task ran correctly but found a real problem (test failures, lint errors, type errors). The script is correct; the code has issues.
- exit 2: a required tool or version is missing (preflight failure). The script is correct; the environment is not ready.
- Do NOT catch errors in a way that changes their exit codes. Let tool exit codes flow through naturally — most tools already exit 1 on failure.
- set -e ensures unexpected command failures (wrong flags, typos, missing files) produce higher exit codes (126, 127, etc.) which signal a script generation error.

Portability:
- Scripts must work on BOTH macOS and Linux.
- Do NOT use GNU-only flags. In particular:
  - Do NOT use grep -P (Perl regex). Use grep -E (extended regex) or sed instead.
  - Do NOT use readlink -f on macOS. Use realpath or a shell workaround.
  - Use "sort -V" for version comparison, or compare numeric components with shell arithmetic.
- For version parsing, prefer: echo "$version" | awk -F. '{print $1, $2}' over grep with regex.

Other rules:
- Output ONLY a valid shell script. No explanation, no markdown fences, no commentary.
- Use commands appropriate for the detected project stack.
- If a task is ambiguous, pick the most common/standard approach.
- The script will be executed in the project root directory.`

// SystemPrompt returns the system prompt for the LLM.
func SystemPrompt() string {
	return systemPrompt
}

// BuildPrompt constructs the full prompt sent to the LLM for a codegen target.
func BuildPrompt(target *parser.Target, ctx *vibecontext.Collected, vars map[string]string) string {
	recipe := parser.SubstituteVars(target.Recipe, vars)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Task: %s\n\n", recipe))
	b.WriteString("Project context:\n\n")
	b.WriteString(ctx.Format())
	b.WriteString("\nGenerate a shell script that accomplishes this task.\n")

	return b.String()
}

// BuildRetryPrompt constructs a prompt for retrying a failed script generation.
func BuildRetryPrompt(target *parser.Target, ctx *vibecontext.Collected, vars map[string]string, failedScript, errorOutput string) string {
	recipe := parser.SubstituteVars(target.Recipe, vars)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Task: %s\n\n", recipe))
	b.WriteString("Project context:\n\n")
	b.WriteString(ctx.Format())
	b.WriteString("\nThe previous script I generated for this task failed.\n\n")
	b.WriteString("Failed script:\n```\n")
	b.WriteString(failedScript)
	b.WriteString("\n```\n\n")
	b.WriteString("Error output:\n```\n")
	if len(errorOutput) > 4000 {
		b.WriteString(errorOutput[len(errorOutput)-4000:])
		b.WriteString("\n(truncated to last 4000 chars)\n")
	} else {
		b.WriteString(errorOutput)
	}
	b.WriteString("\n```\n\n")
	b.WriteString("Fix the script to resolve this error. Follow the same rules as before.\n")

	return b.String()
}
