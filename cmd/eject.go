package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vibefile-dev/vibe/compiled"
	"github.com/vibefile-dev/vibe/config"
	vibecontext "github.com/vibefile-dev/vibe/context"
	"github.com/vibefile-dev/vibe/llm"
	"github.com/vibefile-dev/vibe/parser"
	"github.com/vibefile-dev/vibe/resolver"
	"github.com/vibefile-dev/vibe/skill"
	"github.com/vibefile-dev/vibe/ui"
)

var (
	ejectOutput  string
	ejectCompile bool
)

var ejectCmd = &cobra.Command{
	Use:   "eject",
	Short: "Generate a standard Makefile from compiled Vibefile targets",
	Long: `Eject reads the Vibefile and its compiled shell scripts, then generates a
standard, self-contained Makefile that reproduces the same behaviour without
requiring the vibe CLI or any LLM calls.

Only codegen targets with existing compiled scripts are included. Agent targets
(@mcp) are skipped with a warning.

Use --compile to automatically compile any targets that are missing or stale
before generating the Makefile (requires an API key).`,
	Args: cobra.NoArgs,
	RunE: ejectMakefile,
}

func init() {
	ejectCmd.Flags().StringVarP(&ejectOutput, "output", "o", "", "Write Makefile to a file instead of stdout (e.g. -o Makefile)")
	ejectCmd.Flags().BoolVar(&ejectCompile, "compile", false, "Compile missing/stale targets before ejecting (requires API key)")
	rootCmd.AddCommand(ejectCmd)
}

// compileTarget generates and caches the script for a single codegen target
// without executing it. Returns the compiled script or an error.
func compileTarget(repoRoot string, vf *parser.Vibefile, target *parser.Target, projCfg *config.ProjectConfig) (string, error) {
	model := config.ResolveModel(target.Model, vf.Variables["model"])

	sp := ui.NewSpinner(fmt.Sprintf("Collecting context for %q…", target.Name))
	ctx, err := vibecontext.Collect(repoRoot, target)
	if err != nil {
		sp.Fail(fmt.Sprintf("Context collection failed: %v", err))
		return "", fmt.Errorf("context collection: %w", err)
	}
	sp.Success(fmt.Sprintf("Context collected (%d files)", len(ctx.HashableFiles())))

	contextFiles := ctx.HashableFiles()

	var skillInfo *skill.SkillInfo
	skillRawContent := ""
	if target.HasDirective("skill") {
		skillName := target.DirectiveArgs("skill")
		info, err := skill.Resolve(repoRoot, skillName, projCfg.SkillSources)
		if err != nil {
			return "", fmt.Errorf("skill %q: %w", skillName, err)
		}
		ui.Step(fmt.Sprintf("Skill %q loaded from %s", skillName, info.FilePath))
		skillInfo = info
		skillRawContent = info.RawContent
	}

	// Check if we already have a valid cached script
	lock, err := compiled.LoadLock(repoRoot, target.Name)
	if err == nil {
		valid, _ := compiled.IsValid(lock, target, model, vf.Variables, contextFiles, skillRawContent)
		if valid {
			if script, ok := compiled.Load(repoRoot, target.Name); ok {
				ui.Success(fmt.Sprintf("Using cached script for %q (compiled %s)", target.Name, lock.GeneratedAt.Format("2006-01-02 15:04")))
				return script, nil
			}
		}
	}

	// Need to compile via LLM
	apiKey, err := config.ResolveAPIKey(apiKeyFlag, model)
	if err != nil {
		return "", fmt.Errorf("cannot compile %q: %w", target.Name, err)
	}

	genSp := ui.NewSpinner(fmt.Sprintf("Generating script for %q (%s)…", target.Name, model))

	client := llm.NewClient(apiKey, model)
	var skillEvents []llm.SkillEvent
	client.OnSkillInvoked = func(evt llm.SkillEvent) {
		genSp.Update(fmt.Sprintf("Skill %q invoked by model (iteration %d)…", evt.SkillName, evt.Iteration))
		skillEvents = append(skillEvents, evt)
	}

	prompt := llm.BuildPrompt(target, ctx, vf.Variables)
	var skills []*skill.SkillInfo
	if skillInfo != nil {
		skills = []*skill.SkillInfo{skillInfo}
	}
	script, err := client.Generate(llm.SystemPrompt(), prompt, skills)
	if err != nil {
		genSp.Fail(fmt.Sprintf("LLM generation failed: %v", err))
		return "", fmt.Errorf("LLM generation for %q: %w", target.Name, err)
	}

	genSp.Success(fmt.Sprintf("Script generated for %q", target.Name))
	for _, evt := range skillEvents {
		ui.Info(fmt.Sprintf("used skill %q (tool-use iteration %d)", evt.SkillName, evt.Iteration))
	}

	// Cache the compiled script
	compiledLock := compiled.BuildLock(target, model, vf.Variables, contextFiles, skillRawContent, script)
	if err := compiled.Save(repoRoot, target.Name, script, compiledLock); err != nil {
		ui.Warn(fmt.Sprintf("failed to cache compiled script: %v", err))
	}

	return script, nil
}

// splitPreflight splits a compiled script into preflight checks and the task body.
// It detects the "# Preflight checks" or "# --- preflight ---" section and
// separates it from the rest. Lines like shebang and set -euo pipefail are
// always stripped (handled by Makefile SHELL/.SHELLFLAGS/.ONESHELL).
func splitPreflight(script string) (preflight []string, body []string) {
	lines := strings.Split(strings.TrimRight(script, "\n"), "\n")

	inPreflight := false
	preflightDone := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Always skip shebang and set -e variants
		if strings.HasPrefix(trimmed, "#!/") {
			continue
		}
		if trimmed == "set -euo pipefail" || trimmed == "set -e" || trimmed == "set -eu" || trimmed == "set -euo" {
			continue
		}

		// Detect start of preflight section
		if !preflightDone && !inPreflight && isPreflightHeader(trimmed) {
			inPreflight = true
			continue
		}

		if inPreflight {
			// Preflight section ends when we hit a non-preflight comment
			// (like "# Task:", "# Build", "# --- task ---", "# Run", etc.)
			// or a line that's clearly task logic (not a command check or empty/blank)
			if isTaskHeader(trimmed) {
				inPreflight = false
				preflightDone = true
				body = append(body, line)
				continue
			}
			preflight = append(preflight, line)
			continue
		}

		body = append(body, line)
	}

	return preflight, body
}

// isPreflightHeader returns true if the line marks the start of a preflight section.
func isPreflightHeader(line string) bool {
	lower := strings.ToLower(line)
	return lower == "# preflight checks" ||
		lower == "# --- preflight ---" ||
		lower == "# preflight" ||
		lower == "# --- preflight checks ---"
}

// isTaskHeader returns true if the line looks like a comment header starting the
// actual task body (signaling the end of preflight).
func isTaskHeader(line string) bool {
	if !strings.HasPrefix(line, "#") {
		// Non-comment, non-empty, non-check line = task body
		// But we need to be careful: if/fi/echo/exit lines are still preflight
		if line == "" {
			return false
		}
		return !isPreflightLine(line)
	}
	// It's a comment — check if it looks like a task section header
	lower := strings.ToLower(line)
	if strings.HasPrefix(lower, "# --- preflight") {
		return false
	}
	if lower == "# preflight checks" || lower == "# preflight" {
		return false
	}
	// Any other comment header signals task body
	// e.g. "# Build the binary", "# Task:", "# --- task ---", "# Run all tests"
	return true
}

// isPreflightLine returns true if a non-comment line looks like part of a
// preflight check block (if/then/fi, command -v, exit 2, echo "error:", etc.)
func isPreflightLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || trimmed == "fi" || trimmed == "done" || trimmed == "then" {
		return true
	}
	if strings.HasPrefix(trimmed, "if ") || strings.HasPrefix(trimmed, "if !") {
		return true
	}
	if strings.Contains(trimmed, "command -v") {
		return true
	}
	if strings.Contains(trimmed, "exit 2") {
		return true
	}
	if strings.HasPrefix(trimmed, "echo \"error:") || strings.HasPrefix(trimmed, "echo 'error:") {
		return true
	}
	if strings.HasPrefix(trimmed, "for ") && strings.Contains(trimmed, "; do") {
		return true
	}
	return false
}

func ejectMakefile(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	vf, err := loadVibefile(repoRoot)
	if err != nil {
		return err
	}

	// If --compile is set, compile any missing/stale codegen targets first
	if ejectCompile {
		projCfg, err := config.LoadProjectConfig(repoRoot)
		if err != nil {
			projCfg = &config.ProjectConfig{}
		}

		for _, name := range vf.Order {
			target := vf.Targets[name]
			if target.ExecutionMode() == "agent" {
				continue
			}

			if _, err := compileTarget(repoRoot, vf, target, projCfg); err != nil {
				return fmt.Errorf("compile target %q: %w", name, err)
			}
		}
	}

	// First pass: load all scripts and split them into preflight + body
	type targetScript struct {
		preflight []string
		body      []string
		recipe    string
	}
	scripts := make(map[string]*targetScript)

	for _, name := range vf.Order {
		target := vf.Targets[name]
		if target.ExecutionMode() == "agent" {
			continue
		}

		script, ok := compiled.Load(repoRoot, name)
		if !ok {
			fmt.Fprintf(os.Stderr, "warning: target %q has no compiled script, skipping (run `vibe eject --compile` or `vibe run %s` first)\n", name, name)
			continue
		}

		preflight, body := splitPreflight(script)
		recipe := parser.SubstituteVars(target.Recipe, vf.Variables)
		scripts[name] = &targetScript{preflight: preflight, body: body, recipe: recipe}
	}

	var buf strings.Builder

	buf.WriteString("# Makefile — ejected from Vibefile by `vibe eject`\n")
	buf.WriteString("# This is a standalone Makefile. It does not require the vibe CLI.\n")
	buf.WriteString("SHELL := /bin/bash\n")
	buf.WriteString(".SHELLFLAGS := -euo pipefail -c\n")
	buf.WriteString(".ONESHELL:\n")
	buf.WriteString("\n")

	// Write variables
	hasVars := false
	for _, k := range sortedVarKeys(vf.Variables) {
		v := vf.Variables[k]
		if k == "model" || k == "max_retries" {
			continue
		}
		buf.WriteString(fmt.Sprintf("%s := %s\n", strings.ToUpper(k), v))
		hasVars = true
	}
	if hasVars {
		buf.WriteString("\n")
	}

	// Determine the default target (first in declaration order)
	if len(vf.Order) > 0 {
		buf.WriteString(fmt.Sprintf(".DEFAULT_GOAL := %s\n", vf.Order[0]))
	}

	// Build .PHONY list including preflight targets
	var phonyTargets []string
	for _, name := range vf.Order {
		phonyTargets = append(phonyTargets, name)
		if ts, ok := scripts[name]; ok && len(ts.preflight) > 0 {
			phonyTargets = append(phonyTargets, "_preflight-"+name)
		}
	}
	buf.WriteString(fmt.Sprintf(".PHONY: %s\n", strings.Join(phonyTargets, " ")))
	buf.WriteString("\n")

	skippedAgents := []string{}

	// Write preflight targets first (grouped at the top for clarity)
	hasPreflight := false
	for _, name := range vf.Order {
		ts, ok := scripts[name]
		if !ok || len(ts.preflight) == 0 {
			continue
		}
		if !hasPreflight {
			buf.WriteString("# ── preflight checks ─────────────────────────────────\n\n")
			hasPreflight = true
		}
		buf.WriteString(fmt.Sprintf("_preflight-%s:\n", name))
		for _, line := range ts.preflight {
			converted := convertVarRefs(line, vf.Variables)
			buf.WriteString("\t")
			buf.WriteString(converted)
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}

	if hasPreflight {
		buf.WriteString("# ── targets ──────────────────────────────────────────\n\n")
	}

	// Write main targets
	for _, name := range vf.Order {
		target := vf.Targets[name]

		if target.ExecutionMode() == "agent" {
			skippedAgents = append(skippedAgents, name)
			buf.WriteString(fmt.Sprintf("# %s: skipped (agent mode — requires @mcp, cannot be ejected)\n\n", name))
			continue
		}

		ts, ok := scripts[name]
		if !ok {
			buf.WriteString(fmt.Sprintf("# %s: skipped (no compiled script found)\n\n", name))
			continue
		}

		if ts.recipe != "" {
			buf.WriteString(fmt.Sprintf("# %s\n", ts.recipe))
		}

		// Build dependency list: original deps + preflight if present
		var deps []string
		if len(ts.preflight) > 0 {
			deps = append(deps, "_preflight-"+name)
		}
		deps = append(deps, target.Dependencies...)

		depStr := ""
		if len(deps) > 0 {
			depStr = " " + strings.Join(deps, " ")
		}
		buf.WriteString(fmt.Sprintf("%s:%s\n", name, depStr))

		// Write task body (preflight already extracted)
		for _, line := range ts.body {
			converted := convertVarRefs(line, vf.Variables)
			buf.WriteString("\t")
			buf.WriteString(converted)
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}

	// Help target
	buf.WriteString("help:\n")
	buf.WriteString("\t@echo \"Available targets:\"\n")
	for _, name := range vf.Order {
		target := vf.Targets[name]
		if target.ExecutionMode() == "agent" {
			continue
		}
		if _, ok := scripts[name]; !ok {
			continue
		}
		recipe := parser.SubstituteVars(target.Recipe, vf.Variables)
		if recipe == "" {
			recipe = fmt.Sprintf("(@skill %s)", target.DirectiveArgs("skill"))
		}
		recipe = strings.ReplaceAll(recipe, `"`, `\"`)
		buf.WriteString(fmt.Sprintf("\t@echo \"  %-20s %s\"\n", name, recipe))
	}
	buf.WriteString("\n")

	output := buf.String()

	if ejectOutput != "" {
		if err := os.WriteFile(ejectOutput, []byte(output), 0o644); err != nil {
			return fmt.Errorf("write Makefile: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Makefile written to %s\n", ejectOutput)
		if len(skippedAgents) > 0 {
			fmt.Fprintf(os.Stderr, "warning: skipped agent targets: %s\n", strings.Join(skippedAgents, ", "))
		}
		return nil
	}

	fmt.Print(output)

	if len(skippedAgents) > 0 {
		fmt.Fprintf(os.Stderr, "\nwarning: skipped agent targets: %s\n", strings.Join(skippedAgents, ", "))
	}

	// Verify all targets in dependency chains have compiled scripts
	for _, name := range vf.Order {
		target := vf.Targets[name]
		if target.ExecutionMode() == "agent" {
			continue
		}
		if _, ok := scripts[name]; !ok {
			continue
		}
		order, err := resolver.Resolve(vf, name)
		if err != nil {
			continue
		}
		for _, dep := range order {
			if dep == name {
				continue
			}
			depTarget := vf.Targets[dep]
			if depTarget.ExecutionMode() == "agent" {
				fmt.Fprintf(os.Stderr, "warning: target %q depends on agent target %q which cannot be ejected\n", name, dep)
			}
		}
	}

	return nil
}

// convertVarRefs replaces $(VAR) Vibefile-style variable references with
// $(VAR) Makefile-style references (using uppercased names to match the
// variable declarations at the top of the generated Makefile).
func convertVarRefs(line string, vars map[string]string) string {
	result := line
	for k := range vars {
		if k == "model" || k == "max_retries" {
			continue
		}
		result = strings.ReplaceAll(result, "$("+k+")", "$("+strings.ToUpper(k)+")")
	}
	return result
}

// sortedVarKeys returns variable keys in sorted order for deterministic output.
func sortedVarKeys(vars map[string]string) []string {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	// Simple insertion sort for small maps
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
