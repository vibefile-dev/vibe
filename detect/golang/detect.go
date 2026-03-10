package golang

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/vibefiledev/vibe/detect"
)

func init() { detect.Register(&Detector{}) }

// Detector identifies Go projects by the presence of go.mod or go.work.
type Detector struct{}

func (d *Detector) Name() string { return "go" }

func (d *Detector) Detect(repoRoot string) (*detect.ProjectInfo, bool) {
	slog.Debug("go detector: checking for go.mod", "path", filepath.Join(repoRoot, "go.mod"))

	if info, ok := d.detectFromGoMod(repoRoot, filepath.Join(repoRoot, "go.mod")); ok {
		return info, true
	}

	slog.Debug("go detector: no go.mod at root, checking for go.work", "path", filepath.Join(repoRoot, "go.work"))
	if info, ok := d.detectFromGoWork(repoRoot); ok {
		return info, true
	}

	slog.Debug("go detector: no Go project indicators found")
	return nil, false
}

func (d *Detector) detectFromGoMod(repoRoot, goModPath string) (*detect.ProjectInfo, bool) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		slog.Debug("go detector: could not read file", "path", goModPath, "error", err)
		return nil, false
	}

	slog.Debug("go detector: found go.mod", "path", goModPath)

	info := &detect.ProjectInfo{
		Language:       "go",
		PackageManager: "go",
		Metadata:       make(map[string]string),
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			info.Module = strings.TrimSpace(strings.TrimPrefix(line, "module"))
			info.BinaryName = inferBinaryName(info.Module)
			slog.Debug("go detector: parsed module", "module", info.Module, "binary_name", info.BinaryName)
		}
		if strings.HasPrefix(line, "go ") {
			info.Version = strings.TrimSpace(strings.TrimPrefix(line, "go"))
			slog.Debug("go detector: parsed go version", "version", info.Version)
		}
	}

	info.HasTests = hasTests(repoRoot)
	slog.Debug("go detector: test scan complete", "has_tests", info.HasTests)

	if detect.FileExists(filepath.Join(repoRoot, "main.go")) {
		info.Metadata["has_main"] = "true"
	}

	return info, true
}

func (d *Detector) detectFromGoWork(repoRoot string) (*detect.ProjectInfo, bool) {
	goWorkPath := filepath.Join(repoRoot, "go.work")
	data, err := os.ReadFile(goWorkPath)
	if err != nil {
		return nil, false
	}

	slog.Debug("go detector: found go.work", "path", goWorkPath)

	info := &detect.ProjectInfo{
		Language:       "go",
		PackageManager: "go",
		BinaryName:     filepath.Base(repoRoot),
		Metadata:       map[string]string{"workspace": "go.work"},
	}

	var modules []string
	inUse := false
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "go ") {
			info.Version = strings.TrimSpace(strings.TrimPrefix(line, "go"))
			slog.Debug("go detector: parsed go version from go.work", "version", info.Version)
		}
		if line == "use (" {
			inUse = true
			continue
		}
		if line == ")" {
			inUse = false
			continue
		}
		if inUse {
			dir := strings.TrimSpace(line)
			if dir != "" && !strings.HasPrefix(dir, "//") {
				modules = append(modules, dir)
			}
		}
		if strings.HasPrefix(line, "use ") && !strings.Contains(line, "(") {
			dir := strings.TrimSpace(strings.TrimPrefix(line, "use"))
			if dir != "" {
				modules = append(modules, dir)
			}
		}
	}

	slog.Debug("go detector: workspace modules", "modules", modules)
	info.Modules = modules

	var moduleNames []string
	for _, modDir := range modules {
		subModPath := filepath.Join(repoRoot, modDir, "go.mod")
		subData, err := os.ReadFile(subModPath)
		if err != nil {
			slog.Debug("go detector: no go.mod in workspace member", "dir", modDir)
			continue
		}
		for _, line := range strings.Split(string(subData), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				mod := strings.TrimSpace(strings.TrimPrefix(line, "module"))
				moduleNames = append(moduleNames, mod)
				slog.Debug("go detector: found workspace member module", "dir", modDir, "module", mod)
				break
			}
		}
	}

	if len(moduleNames) > 0 {
		info.Module = commonModulePrefix(moduleNames)
		slog.Debug("go detector: derived common module prefix", "prefix", info.Module, "all_modules", moduleNames)
	}

	info.HasTests = hasTests(repoRoot)
	slog.Debug("go detector: test scan complete", "has_tests", info.HasTests)

	for _, modDir := range modules {
		if detect.FileExists(filepath.Join(repoRoot, modDir, "main.go")) {
			info.Metadata["has_main"] = "true"
			break
		}
	}

	return info, true
}

func commonModulePrefix(modules []string) string {
	if len(modules) == 0 {
		return ""
	}
	if len(modules) == 1 {
		return modules[0]
	}
	parts := strings.Split(modules[0], "/")
	for _, mod := range modules[1:] {
		mp := strings.Split(mod, "/")
		n := len(parts)
		if len(mp) < n {
			n = len(mp)
		}
		match := 0
		for i := 0; i < n; i++ {
			if parts[i] != mp[i] {
				break
			}
			match = i + 1
		}
		parts = parts[:match]
	}
	return strings.Join(parts, "/")
}

func inferBinaryName(modulePath string) string {
	parts := strings.Split(modulePath, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func hasTests(repoRoot string) bool {
	found := false
	filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return filepath.SkipDir
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == "node_modules" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), "_test.go") {
			found = true
			return filepath.SkipDir
		}
		return nil
	})
	return found
}
