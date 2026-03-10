package helm

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/vibefile-dev/vibe/detect"
)

func init() { detect.RegisterAddon(&Addon{}) }

// Addon detects Helm charts and contributes chart management targets.
type Addon struct{}

func (a *Addon) Name() string { return "helm" }

func (a *Addon) Detect(repoRoot string) *detect.AddonResult {
	chartPath := findChart(repoRoot)
	if chartPath == "" {
		return nil
	}
	slog.Debug("helm addon: Chart.yaml found", "path", chartPath)

	chartDir := filepath.Dir(chartPath)
	rel, _ := filepath.Rel(repoRoot, chartDir)
	if rel == "" || rel == "." {
		rel = "."
	}

	targets := []detect.TemplateTarget{
		{
			Name:    "helm-lint",
			Section: "helm",
			Recipe:  "run helm lint on the chart in " + rel + " to validate templates and values",
		},
		{
			Name:    "helm-template",
			Section: "helm",
			Recipe:  "render the Helm chart templates in " + rel + " locally and print the output for review",
		},
		{
			Name:         "helm-package",
			Section:      "helm",
			Dependencies: []string{"helm-lint"},
			Recipe:       "package the Helm chart in " + rel + " into a .tgz archive",
		},
	}

	return &detect.AddonResult{
		Label:   "Helm",
		Targets: targets,
	}
}

func findChart(repoRoot string) string {
	// Check root
	if detect.FileExists(filepath.Join(repoRoot, "Chart.yaml")) {
		return filepath.Join(repoRoot, "Chart.yaml")
	}

	// Check common subdirectories: chart/, charts/, helm/, deploy/
	for _, dir := range []string{"chart", "charts", "helm", "deploy"} {
		candidate := filepath.Join(repoRoot, dir, "Chart.yaml")
		if detect.FileExists(candidate) {
			return candidate
		}

		// One level deeper (charts/<name>/Chart.yaml)
		sub := filepath.Join(repoRoot, dir)
		entries, err := os.ReadDir(sub)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				candidate = filepath.Join(sub, e.Name(), "Chart.yaml")
				if detect.FileExists(candidate) {
					return candidate
				}
			}
		}
	}

	return ""
}
