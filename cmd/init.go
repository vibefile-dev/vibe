package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vibefile-dev/vibe/detect"
	_ "github.com/vibefile-dev/vibe/detect/cloudflare"
	_ "github.com/vibefile-dev/vibe/detect/docker"
	_ "github.com/vibefile-dev/vibe/detect/fly"
	_ "github.com/vibefile-dev/vibe/detect/golang"
	_ "github.com/vibefile-dev/vibe/detect/helm"
	_ "github.com/vibefile-dev/vibe/detect/makefile"
	_ "github.com/vibefile-dev/vibe/detect/nextjs"
	_ "github.com/vibefile-dev/vibe/detect/vercel"
	"github.com/vibefile-dev/vibe/ui"
)

var (
	initLanguage string
	initForce    bool
	initEmpty    bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Detect project type and generate a Vibefile",
	Long: `Detect the project's language, framework, and infrastructure from manifest
files (go.mod, go.work, package.json, Dockerfile, Chart.yaml, wrangler.toml,
Makefile, etc.) and generate a Vibefile with sensible default targets.

For monorepos, subdirectories are scanned automatically when no language is
detected at the root. Each subdirectory's targets are prefixed with the
directory name (e.g. go-build, ui-dev). No LLM call required.

Use --empty to create a minimal skeleton Vibefile without auto-detection,
suitable for projects where you want to define targets manually.`,
	Args: cobra.NoArgs,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&initLanguage, "language", "", "Skip detection and use a specific language template (e.g. go)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite existing Vibefile")
	initCmd.Flags().BoolVar(&initEmpty, "empty", false, "Create a minimal skeleton Vibefile without auto-detected targets")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	slog.Debug("init: starting", "repo_root", repoRoot, "language_flag", initLanguage, "force", initForce)

	vibePath := filepath.Join(repoRoot, "Vibefile")
	if !initForce {
		if _, err := os.Stat(vibePath); err == nil {
			return fmt.Errorf("Vibefile already exists (use --force to overwrite)")
		}
	}

	// --- Empty skeleton ---
	if initEmpty {
		return initEmptyVibefile(repoRoot, vibePath)
	}

	// --- Explicit language flag: single-project mode ---
	if initLanguage != "" {
		return initSingleProjectExplicit(repoRoot, vibePath)
	}

	// --- Auto-detect: try root first, then scan subdirectories ---
	sp := ui.NewSpinner("Detecting project type…")
	project := detect.DetectLanguage(repoRoot)

	if project != nil {
		sp.Success(formatProjectDetected(project))
		return initSingleProject(repoRoot, vibePath, project)
	}

	// No language at root — try monorepo scan
	sp.Update("Scanning subdirectories…")
	subProjects := detect.ScanSubdirectories(repoRoot)

	if len(subProjects) == 0 {
		sp.Fail("Could not detect project type")
		available := detect.AvailableLanguages()
		return fmt.Errorf(
			"could not detect project type (checked root and subdirectories)\n"+
				"  Use --language to specify manually: vibe init --language <lang>\n"+
				"  Available: %s",
			strings.Join(available, ", "),
		)
	}

	sp.Success(fmt.Sprintf("Monorepo detected — %d sub-projects found", len(subProjects)))
	return initMonorepo(repoRoot, vibePath, subProjects)
}

// initEmptyVibefile creates a minimal skeleton Vibefile with no auto-detected targets.
func initEmptyVibefile(repoRoot, vibePath string) error {
	projectName := filepath.Base(repoRoot)
	slog.Debug("init: empty mode", "project_name", projectName)

	content := emptyVibefileContent(projectName)

	if err := os.WriteFile(vibePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write Vibefile: %w", err)
	}

	ui.Success("Empty Vibefile created")
	ui.Info("Edit the Vibefile to add your targets, then run \"vibe list\" to verify")

	return nil
}

func emptyVibefileContent(projectName string) string {
	return fmt.Sprintf(`model = claude-sonnet-4-6
name  = %s

# Add your targets below. Each target has a name, an optional dependency
# list, and a plain-English recipe describing what the task should do.
#
# Example:
#
# build:
#     "compile the project for production"
#
# test:
#     "run all tests with verbose output"
#
# deploy: test build:
#     "deploy to production and verify health"
#     @require clean git status
`, projectName)
}

// initSingleProjectExplicit handles --language flag
func initSingleProjectExplicit(repoRoot, vibePath string) error {
	sp := ui.NewSpinner(fmt.Sprintf("Loading %s template…", initLanguage))
	project, ok := detect.DetectByLanguage(repoRoot, initLanguage)
	if !ok {
		slog.Debug("init: language detector did not match, using fallback", "language", initLanguage)
		project = &detect.ProjectInfo{
			Language:       initLanguage,
			PackageManager: initLanguage,
			BinaryName:     filepath.Base(repoRoot),
			Metadata:       make(map[string]string),
		}
	}
	sp.Success(fmt.Sprintf("Using %s template", initLanguage))
	return initSingleProject(repoRoot, vibePath, project)
}

// initSingleProject generates a Vibefile for a single-project repo (existing behavior).
func initSingleProject(repoRoot, vibePath string, project *detect.ProjectInfo) error {
	slog.Debug("init: single project mode",
		"language", project.Language,
		"module", project.Module,
		"version", project.Version,
	)

	addonResults := detect.DetectAddons(repoRoot)
	printAddonResults(addonResults)

	tmpl, err := detect.ResolveTemplate(repoRoot, project.Language, project)
	if err != nil {
		return fmt.Errorf("template resolution: %w", err)
	}

	detect.MergeAddons(tmpl, addonResults)

	return writeVibefile(vibePath, tmpl)
}

// initMonorepo generates a Vibefile for a monorepo with multiple sub-projects.
func initMonorepo(repoRoot, vibePath string, subProjects []detect.SubProject) error {
	tmpl := &detect.Template{
		Variables: []detect.TemplateVariable{
			{Key: "model", Value: "claude-sonnet-4-6"},
			{Key: "name", Value: filepath.Base(repoRoot)},
		},
	}

	// Process each sub-project
	for _, sp := range subProjects {
		ui.Success(fmt.Sprintf("%s/ — %s project%s", sp.Dir, capitalize(sp.Project.Language), formatVersion(sp.Project)))

		subTmpl, err := detect.ResolveSubProjectTemplate(repoRoot, sp)
		if err != nil {
			ui.Warn(fmt.Sprintf("skipping %s/: %v", sp.Dir, err))
			continue
		}

		// Merge sub-project targets into main template
		tmpl.Targets = append(tmpl.Targets, subTmpl.Targets...)

		// Run addons inside the sub-project directory
		subAddons := detect.DetectAddonsInDir(repoRoot, sp.Dir)
		detect.MergeAddons(tmpl, subAddons)
		printAddonResults(subAddons)
	}

	// Run addons at root level (root Makefile, root Dockerfile, Helm, etc.)
	rootAddons := detect.DetectAddons(repoRoot)
	detect.MergeAddons(tmpl, rootAddons)
	printAddonResults(rootAddons)

	return writeVibefile(vibePath, tmpl)
}

func writeVibefile(vibePath string, tmpl *detect.Template) error {
	content := detect.Generate(tmpl)
	slog.Debug("init: generated Vibefile content", "length", len(content))

	if err := os.WriteFile(vibePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write Vibefile: %w", err)
	}

	targetNames := make([]string, len(tmpl.Targets))
	for i, t := range tmpl.Targets {
		targetNames[i] = t.Name
	}

	ui.Success(fmt.Sprintf("Vibefile created with %d targets", len(tmpl.Targets)))
	ui.Info(strings.Join(targetNames, ", "))
	fmt.Fprintln(os.Stderr)
	ui.Info("Run \"vibe list\" to see targets or \"vibe run <target>\" to execute one")

	return nil
}

func printAddonResults(results []*detect.AddonResult) {
	for _, r := range results {
		targetNames := make([]string, len(r.Targets))
		for i, t := range r.Targets {
			targetNames[i] = t.Name
		}
		ui.Success(fmt.Sprintf("%s detected → %s", r.Label, strings.Join(targetNames, ", ")))
	}
}

func formatProjectDetected(project *detect.ProjectInfo) string {
	versionStr := ""
	if project.Version != "" {
		versionStr = fmt.Sprintf(" %s", project.Version)
	}
	moduleStr := ""
	if project.Module != "" {
		moduleStr = fmt.Sprintf(", module: %s", project.Module)
	}
	return fmt.Sprintf("%s project detected (%s%s%s)", capitalize(project.Language), project.Language, versionStr, moduleStr)
}

func formatVersion(project *detect.ProjectInfo) string {
	if project.Version != "" {
		return fmt.Sprintf(" (%s %s)", project.Language, project.Version)
	}
	return ""
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
