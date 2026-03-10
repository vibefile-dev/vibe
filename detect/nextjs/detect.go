package nextjs

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/vibefile-dev/vibe/detect"
)

func init() { detect.Register(&Detector{}) }

// Detector identifies Next.js projects by package.json + next dependency.
type Detector struct{}

func (d *Detector) Name() string { return "nextjs" }

func (d *Detector) Detect(repoRoot string) (*detect.ProjectInfo, bool) {
	pkgPath := filepath.Join(repoRoot, "package.json")
	slog.Debug("nextjs detector: checking for package.json", "path", pkgPath)

	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil, false
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		slog.Debug("nextjs detector: could not parse package.json", "error", err)
		return nil, false
	}

	nextVersion := pkg.depVersion("next")
	if nextVersion == "" {
		slog.Debug("nextjs detector: 'next' not found in dependencies")
		return nil, false
	}

	slog.Debug("nextjs detector: Next.js project found", "next_version", nextVersion)

	info := &detect.ProjectInfo{
		Language:       "nextjs",
		Framework:      "next",
		PackageManager: detectPackageManager(repoRoot),
		BinaryName:     pkg.Name,
		Module:         pkg.Name,
		Metadata:       make(map[string]string),
	}

	info.Version = strings.TrimPrefix(nextVersion, "^")
	info.Version = strings.TrimPrefix(info.Version, "~")

	if pkg.Name != "" {
		slog.Debug("nextjs detector: package name", "name", pkg.Name)
	}

	// TypeScript detection
	if detect.FileExists(filepath.Join(repoRoot, "tsconfig.json")) {
		info.Metadata["typescript"] = "true"
		slog.Debug("nextjs detector: TypeScript detected")
	}

	// Router type detection
	if detect.FileExists(filepath.Join(repoRoot, "app")) || detect.FileExists(filepath.Join(repoRoot, "src", "app")) {
		info.Metadata["router"] = "app"
		slog.Debug("nextjs detector: App Router detected")
	} else if detect.FileExists(filepath.Join(repoRoot, "pages")) || detect.FileExists(filepath.Join(repoRoot, "src", "pages")) {
		info.Metadata["router"] = "pages"
		slog.Debug("nextjs detector: Pages Router detected")
	}

	// Tailwind CSS
	for _, name := range []string{"tailwind.config.js", "tailwind.config.ts", "tailwind.config.mjs"} {
		if detect.FileExists(filepath.Join(repoRoot, name)) {
			info.Metadata["tailwind"] = "true"
			slog.Debug("nextjs detector: Tailwind CSS detected")
			break
		}
	}

	// Testing framework
	if pkg.hasDep("vitest") {
		info.Metadata["test_framework"] = "vitest"
		info.HasTests = true
	} else if pkg.hasDep("jest") {
		info.Metadata["test_framework"] = "jest"
		info.HasTests = true
	}
	if pkg.hasDep("playwright") || pkg.hasDep("@playwright/test") {
		info.Metadata["e2e_framework"] = "playwright"
		info.HasTests = true
	} else if pkg.hasDep("cypress") {
		info.Metadata["e2e_framework"] = "cypress"
		info.HasTests = true
	}
	if info.HasTests {
		slog.Debug("nextjs detector: tests detected",
			"test_framework", info.Metadata["test_framework"],
			"e2e_framework", info.Metadata["e2e_framework"],
		)
	}

	// Prettier
	if pkg.hasDep("prettier") || detect.FileExists(filepath.Join(repoRoot, ".prettierrc")) ||
		detect.FileExists(filepath.Join(repoRoot, ".prettierrc.json")) ||
		detect.FileExists(filepath.Join(repoRoot, "prettier.config.js")) ||
		detect.FileExists(filepath.Join(repoRoot, "prettier.config.mjs")) {
		info.Metadata["prettier"] = "true"
		slog.Debug("nextjs detector: Prettier detected")
	}

	// Detect scripts in package.json
	if pkg.Scripts["lint"] != "" {
		info.Metadata["has_lint_script"] = "true"
	}
	if pkg.Scripts["test"] != "" {
		info.Metadata["has_test_script"] = "true"
		info.HasTests = true
	}

	slog.Debug("nextjs detector: detection complete",
		"name", info.BinaryName,
		"version", info.Version,
		"package_manager", info.PackageManager,
		"has_tests", info.HasTests,
	)

	return info, true
}

type packageJSON struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func (p *packageJSON) depVersion(name string) string {
	if v, ok := p.Dependencies[name]; ok {
		return v
	}
	if v, ok := p.DevDependencies[name]; ok {
		return v
	}
	return ""
}

func (p *packageJSON) hasDep(name string) bool {
	return p.depVersion(name) != ""
}

func detectPackageManager(repoRoot string) string {
	switch {
	case detect.FileExists(filepath.Join(repoRoot, "pnpm-lock.yaml")):
		return "pnpm"
	case detect.FileExists(filepath.Join(repoRoot, "yarn.lock")):
		return "yarn"
	case detect.FileExists(filepath.Join(repoRoot, "bun.lockb")) || detect.FileExists(filepath.Join(repoRoot, "bun.lock")):
		return "bun"
	default:
		return "npm"
	}
}
