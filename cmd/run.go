package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/vibefiledev/vibe/compiled"
	"github.com/vibefiledev/vibe/config"
	vibecontext "github.com/vibefiledev/vibe/context"
	"github.com/vibefiledev/vibe/executor"
	"github.com/vibefiledev/vibe/llm"
	"github.com/vibefiledev/vibe/parser"
	"github.com/vibefiledev/vibe/resolver"
	"github.com/vibefiledev/vibe/ui"
)

const defaultMaxRetries = 1

var (
	dryRun       bool
	recompile    bool
	recompileAll bool
	noRetry      bool
	cachedOnly   bool
)

var runCmd = &cobra.Command{
	Use:   "run <target>",
	Short: "Run a target and its dependencies",
	Args:  cobra.ExactArgs(1),
	RunE:  runTarget,
}

func init() {
	runCmd.Flags().BoolVar(&dryRun, "dry", false, "Print what would be executed without running")
	runCmd.Flags().BoolVar(&recompile, "recompile", false, "Force LLM recompile for this target")
	runCmd.Flags().BoolVar(&recompileAll, "recompile-all", false, "Force LLM recompile for this target and all dependencies")
	runCmd.Flags().BoolVar(&noRetry, "no-retry", false, "Disable auto-retry on script errors")
	runCmd.Flags().BoolVar(&cachedOnly, "cached-only", false, "Only use cached scripts, never call the LLM (for CI)")
	rootCmd.AddCommand(runCmd)
}

func runTarget(cmd *cobra.Command, args []string) error {
	targetName := args[0]

	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	vf, err := loadVibefile(repoRoot)
	if err != nil {
		return err
	}

	order, err := resolver.Resolve(vf, targetName)
	if err != nil {
		return err
	}

	ui.DependencyChain(order)

	for i, name := range order {
		target := vf.Targets[name]

		ui.TargetHeader(name, i+1, len(order))

		if target.ExecutionMode() == "agent" {
			ui.Warn(fmt.Sprintf("%q — agent mode (@mcp) not yet supported, skipping", name))
			continue
		}
		if target.ExecutionMode() == "skill" && target.Recipe == "" {
			ui.Warn(fmt.Sprintf("%q — @skill resolution not yet supported, skipping", name))
			continue
		}

		forceRecompile := recompileAll || (recompile && name == targetName)

		if err := runCodegenTarget(repoRoot, vf, target, forceRecompile); err != nil {
			return fmt.Errorf("target %q failed: %w", name, err)
		}
	}

	ui.FinalSuccess("All targets completed")
	return nil
}

func runCodegenTarget(repoRoot string, vf *parser.Vibefile, target *parser.Target, forceRecompile bool) error {
	start := time.Now()
	model := config.ResolveModel(target.Model, vf.Variables["model"])

	sp := ui.NewSpinner(fmt.Sprintf("Collecting context for %q…", target.Name))
	ctx, err := vibecontext.Collect(repoRoot, target)
	if err != nil {
		sp.Fail(fmt.Sprintf("Context collection failed: %v", err))
		return fmt.Errorf("context collection: %w", err)
	}
	sp.Success(fmt.Sprintf("Context collected (%d files)", len(ctx.HashableFiles())))

	contextFiles := ctx.HashableFiles()
	useCachedOnly := cachedOnly || isCIEnvironment()
	if forceRecompile {
		useCachedOnly = false
	}
	script, fromCache, err := resolveScript(repoRoot, vf, target, model, contextFiles, ctx, forceRecompile, useCachedOnly)
	if err != nil {
		return err
	}

	if !fromCache || verbose {
		ui.PrintScript(target.Name, script, fromCache)
	}

	if dryRun {
		if fromCache && !verbose {
			ui.PrintScript(target.Name, script, fromCache)
		}
		ui.Info("dry run — not executed")
		return nil
	}

	ui.Warn("running without sandbox (container execution not yet implemented)")

	maxRetries := resolveMaxRetries(vf)
	if noRetry || useCachedOnly {
		maxRetries = 0
	}

	return executeWithRetry(repoRoot, vf, target, model, contextFiles, ctx, script, fromCache, maxRetries, start)
}

func executeWithRetry(
	repoRoot string,
	vf *parser.Vibefile,
	target *parser.Target,
	model string,
	contextFiles map[string]string,
	ctx *vibecontext.Collected,
	script string,
	fromCache bool,
	maxRetries int,
	start time.Time,
) error {
	attempt := 0

	for {
		sp := ui.NewSpinner(fmt.Sprintf("Executing %q…", target.Name))
		result := executor.Run(script, repoRoot)

		if result.IsSuccess() {
			elapsed := time.Since(start).Round(time.Millisecond * 100)
			sp.Success(fmt.Sprintf("%s completed %s", target.Name, formatDuration(elapsed)))
			cacheScript(repoRoot, vf, target, model, contextFiles, script)
			return nil
		}

		if result.IsLegitimateFailure() {
			elapsed := time.Since(start).Round(time.Millisecond * 100)
			sp.Fail(fmt.Sprintf("%s — task found a problem (exit 1) %s", target.Name, formatDuration(elapsed)))
			cacheScript(repoRoot, vf, target, model, contextFiles, script)
			return fmt.Errorf("exit code 1")
		}

		if result.IsPreconditionFailure() {
			elapsed := time.Since(start).Round(time.Millisecond * 100)
			sp.Fail(fmt.Sprintf("%s — missing prerequisite (exit 2) %s", target.Name, formatDuration(elapsed)))
			cacheScript(repoRoot, vf, target, model, contextFiles, script)
			return fmt.Errorf("prerequisite not met (exit 2)")
		}

		// Exit 3+ — script error. Try to auto-retry.
		sp.Stop()
		if attempt < maxRetries {
			attempt++
			retrySp := ui.NewSpinner(fmt.Sprintf("Script error (exit %d) — retrying with error context (%d/%d)…",
				result.ExitCode, attempt, maxRetries))

			apiKey, err := config.ResolveAPIKey(apiKeyFlag, model)
			if err != nil {
				retrySp.Fail("Could not resolve API key for retry")
				return err
			}

			client := llm.NewClient(apiKey, model)
			retryPrompt := llm.BuildRetryPrompt(target, ctx, vf.Variables, script, result.CombinedOutput())
			newScript, err := client.Generate(llm.SystemPrompt(), retryPrompt)
			if err != nil {
				retrySp.Fail(fmt.Sprintf("LLM retry failed: %v", err))
				return fmt.Errorf("LLM retry failed: %w", err)
			}

			retrySp.Success(fmt.Sprintf("Retry script generated (attempt %d/%d)", attempt, maxRetries))
			ui.PrintScript(target.Name, newScript, false)
			script = newScript
			fromCache = false
			continue
		}

		elapsed := time.Since(start).Round(time.Millisecond * 100)
		if maxRetries > 0 {
			ui.Fail(fmt.Sprintf("%s — script error (exit %d), retries exhausted %s", target.Name, result.ExitCode, formatDuration(elapsed)))
		} else {
			ui.Fail(fmt.Sprintf("%s — script error (exit %d) %s", target.Name, result.ExitCode, formatDuration(elapsed)))
		}
		return fmt.Errorf("script error (exit %d)", result.ExitCode)
	}
}

func cacheScript(repoRoot string, vf *parser.Vibefile, target *parser.Target, model string, contextFiles map[string]string, script string) {
	lock := compiled.BuildLock(target, model, vf.Variables, contextFiles, script)
	if err := compiled.Save(repoRoot, target.Name, script, lock); err != nil {
		ui.Warn(fmt.Sprintf("failed to cache compiled script: %v", err))
	}
}

func resolveScript(
	repoRoot string,
	vf *parser.Vibefile,
	target *parser.Target,
	model string,
	contextFiles map[string]string,
	ctx *vibecontext.Collected,
	forceRecompile bool,
	cachedOnly bool,
) (string, bool, error) {

	if !forceRecompile {
		lock, err := compiled.LoadLock(repoRoot, target.Name)
		if err == nil {
			valid, reason := compiled.IsValid(lock, target, model, vf.Variables, contextFiles)
			if valid {
				if script, ok := compiled.Load(repoRoot, target.Name); ok {
					if compiled.IsHandEdited(repoRoot, target.Name, lock) {
						ui.Warn(fmt.Sprintf("%s.sh has been hand-edited (use --recompile to regenerate)", target.Name))
					}
					ui.Success(fmt.Sprintf("Using cached script (compiled %s)", lock.GeneratedAt.Format("2006-01-02 15:04")))
					return script, true, nil
				}
			}

			if cachedOnly {
				return "", false, fmt.Errorf(
					"cached script is stale (%s) and --cached-only is active\n"+
						"  Run locally:  vibe run %s --recompile\n"+
						"  Then commit:  .vibe/compiled/%s.sh and .vibe/compiled/%s.lock",
					reason, target.Name, target.Name, target.Name,
				)
			}
			ui.Info(fmt.Sprintf("recompiling: %s", reason))
		} else {
			if cachedOnly {
				return "", false, fmt.Errorf(
					"no cached script for %q and --cached-only is active\n"+
						"  Run locally:  vibe run %s\n"+
						"  Then commit:  .vibe/compiled/%s.sh and .vibe/compiled/%s.lock",
					target.Name, target.Name, target.Name, target.Name,
				)
			}
		}
	} else {
		ui.Info("forced recompile")
	}

	apiKey, err := config.ResolveAPIKey(apiKeyFlag, model)
	if err != nil {
		return "", false, fmt.Errorf(
			"%w\n  Hint: if this is a CI environment, run `vibe run %s --recompile` locally and commit .vibe/compiled/",
			err, target.Name,
		)
	}

	sp := ui.NewSpinner(fmt.Sprintf("Generating script for %q (%s)…", target.Name, model))

	client := llm.NewClient(apiKey, model)
	prompt := llm.BuildPrompt(target, ctx, vf.Variables)
	script, err := client.Generate(llm.SystemPrompt(), prompt)
	if err != nil {
		sp.Fail(fmt.Sprintf("LLM generation failed: %v", err))
		return "", false, fmt.Errorf("LLM generation: %w", err)
	}

	sp.Success("Script generated")
	return script, false, nil
}

func resolveMaxRetries(vf *parser.Vibefile) int {
	if v, ok := vf.Variables["max_retries"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return defaultMaxRetries
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("(%dms)", d.Milliseconds())
	}
	return fmt.Sprintf("(%.1fs)", d.Seconds())
}

// isCIEnvironment returns true if we're running inside a CI system.
func isCIEnvironment() bool {
	ci := os.Getenv("CI")
	return ci == "true" || ci == "1"
}
