package detect

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Detector identifies a project's language and tooling from the repo root.
type Detector interface {
	Name() string
	Detect(repoRoot string) (*ProjectInfo, bool)
}

// Addon detects a tool, platform, or infrastructure component and contributes
// its own targets to the generated Vibefile. Each addon is self-contained —
// detection and target generation live in the same package.
type Addon interface {
	Name() string
	Detect(repoRoot string) *AddonResult // nil if not detected
}

var detectors []Detector
var addons []Addon

// Register adds a language/project detector to the registry.
func Register(d Detector) {
	detectors = append(detectors, d)
}

// RegisterAddon adds an addon detector to the registry.
func RegisterAddon(a Addon) {
	addons = append(addons, a)
}

// DetectLanguage runs all registered language detectors and returns the first match.
func DetectLanguage(repoRoot string) *ProjectInfo {
	slog.Debug("running language detectors", "repo_root", repoRoot, "count", len(detectors))

	for _, d := range detectors {
		slog.Debug("running language detector", "detector", d.Name())
		if p, ok := d.Detect(repoRoot); ok {
			slog.Debug("language detected", "detector", d.Name(), "language", p.Language)
			return p
		}
		slog.Debug("detector did not match", "detector", d.Name())
	}

	return nil
}

// DetectAddons runs all registered addons and returns results for those that matched.
func DetectAddons(repoRoot string) []*AddonResult {
	slog.Debug("running addon detectors", "repo_root", repoRoot, "count", len(addons))

	var results []*AddonResult
	for _, a := range addons {
		slog.Debug("running addon", "addon", a.Name())
		if r := a.Detect(repoRoot); r != nil {
			slog.Debug("addon matched", "addon", a.Name(), "label", r.Label, "targets", len(r.Targets))
			results = append(results, r)
		}
	}

	return results
}

// ScanSubdirectories walks immediate children of repoRoot and runs all
// language detectors against each directory. Returns all matches as SubProjects.
func ScanSubdirectories(repoRoot string) []SubProject {
	slog.Debug("scanning subdirectories for projects", "repo_root", repoRoot)

	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		slog.Debug("could not read directory", "error", err)
		return nil
	}

	skipDirs := map[string]bool{
		".git": true, ".github": true, ".vscode": true, ".idea": true,
		"node_modules": true, "vendor": true, "dist": true, ".venv": true,
		"__pycache__": true, ".next": true, "target": true,
	}

	var results []SubProject
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || skipDirs[name] {
			continue
		}

		subDir := filepath.Join(repoRoot, name)
		slog.Debug("scanning subdirectory", "dir", name)

		for _, d := range detectors {
			if p, ok := d.Detect(subDir); ok {
				slog.Debug("project found in subdirectory",
					"dir", name, "detector", d.Name(), "language", p.Language)
				results = append(results, SubProject{Dir: name, Project: p})
				break
			}
		}
	}

	slog.Debug("subdirectory scan complete", "projects_found", len(results))
	return results
}

// DetectAddonsInDir runs all addons against a specific directory and optionally
// prefixes target names and recipe context when relDir is non-empty.
func DetectAddonsInDir(repoRoot, relDir string) []*AddonResult {
	dir := repoRoot
	if relDir != "" {
		dir = filepath.Join(repoRoot, relDir)
	}
	slog.Debug("running addon detectors in dir", "dir", dir, "rel", relDir, "count", len(addons))

	var results []*AddonResult
	for _, a := range addons {
		if r := a.Detect(dir); r != nil {
			if relDir != "" {
				PrefixAddonResult(r, relDir)
			}
			slog.Debug("addon matched in dir", "addon", a.Name(), "dir", relDir, "label", r.Label, "targets", len(r.Targets))
			results = append(results, r)
		}
	}
	return results
}

// PrefixAddonResult prepends a directory prefix to all target names and
// adjusts recipes with directory context.
func PrefixAddonResult(r *AddonResult, dir string) {
	r.Label = dir + "/" + r.Label
	for i := range r.Targets {
		r.Targets[i].Name = dir + "-" + r.Targets[i].Name
		for j := range r.Targets[i].Dependencies {
			r.Targets[i].Dependencies[j] = dir + "-" + r.Targets[i].Dependencies[j]
		}
		if r.Targets[i].Recipe != "" {
			r.Targets[i].Recipe = "in the " + dir + "/ directory, " + r.Targets[i].Recipe
		}
	}
}

// DetectByLanguage runs only the detector matching the given language name.
func DetectByLanguage(repoRoot, language string) (*ProjectInfo, bool) {
	for _, d := range detectors {
		if d.Name() == language {
			return d.Detect(repoRoot)
		}
	}
	return nil, false
}

// AvailableLanguages returns the names of all registered detectors.
func AvailableLanguages() []string {
	names := make([]string, len(detectors))
	for i, d := range detectors {
		names[i] = d.Name()
	}
	return names
}
