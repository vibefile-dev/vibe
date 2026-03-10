package makefile

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vibefiledev/vibe/detect"
)

func init() { detect.RegisterAddon(&Addon{}) }

// Addon detects Makefiles and contributes targets that wrap existing make targets.
type Addon struct{}

func (a *Addon) Name() string { return "makefile" }

func (a *Addon) Detect(repoRoot string) *detect.AddonResult {
	var makefilePath string
	for _, name := range []string{"Makefile", "GNUmakefile", "makefile"} {
		candidate := filepath.Join(repoRoot, name)
		if detect.FileExists(candidate) {
			makefilePath = candidate
			break
		}
	}
	if makefilePath == "" {
		return nil
	}

	slog.Debug("makefile addon: found", "path", makefilePath)

	data, err := os.ReadFile(makefilePath)
	if err != nil {
		slog.Debug("makefile addon: could not read", "error", err)
		return nil
	}

	targets := parseTargets(string(data))
	if len(targets) == 0 {
		slog.Debug("makefile addon: no user targets found")
		return nil
	}

	slog.Debug("makefile addon: targets extracted", "count", len(targets))

	templateTargets := make([]detect.TemplateTarget, 0, len(targets))
	for _, name := range targets {
		templateTargets = append(templateTargets, detect.TemplateTarget{
			Name:    "make-" + name,
			Section: "makefile",
			Recipe:  fmt.Sprintf("run the existing Makefile target '%s' by executing make %s", name, name),
		})
	}

	return &detect.AddonResult{
		Label:   "Makefile",
		Targets: templateTargets,
	}
}

// targetLine matches lines like "target-name:" or "target-name: dep1 dep2"
// but not variable assignments (lines with "=") or recipe lines (starting with tab).
var targetLine = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9_-]*)\s*:`)

func parseTargets(content string) []string {
	seen := make(map[string]bool)
	phony := parsePhonyTargets(content)
	var targets []string

	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " ") {
			continue
		}
		if strings.Contains(line, "=") && !strings.Contains(line, ":=") {
			continue
		}
		// Skip special targets and .PHONY declarations
		if strings.HasPrefix(line, ".") {
			continue
		}

		m := targetLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		name := m[1]
		if seen[name] {
			continue
		}
		if skipTarget(name) {
			continue
		}

		seen[name] = true
		targets = append(targets, name)
	}

	// If there are .PHONY targets, prefer only those as they represent
	// the intended user-facing targets. Fall back to all targets if
	// there are no .PHONY declarations.
	if len(phony) > 0 {
		var filtered []string
		for _, t := range targets {
			if phony[t] {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) > 0 {
			return filtered
		}
	}

	return targets
}

func parsePhonyTargets(content string) map[string]bool {
	phony := make(map[string]bool)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, ".PHONY") {
			continue
		}
		// .PHONY: target1 target2 ...
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		for _, t := range strings.Fields(parts[1]) {
			phony[t] = true
		}
	}
	return phony
}

func skipTarget(name string) bool {
	// Skip internal/conventional targets
	switch name {
	case "all", "default", "FORCE":
		return true
	}
	return strings.HasPrefix(name, "_")
}
