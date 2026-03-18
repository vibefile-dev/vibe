package python

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/vibefile-dev/vibe/detect"
)

func init() { detect.Register(&Detector{}) }

// Detector identifies Python projects by the presence of pyproject.toml,
// setup.py, setup.cfg, or requirements.txt.
type Detector struct{}

func (d *Detector) Name() string { return "python" }

func (d *Detector) Detect(repoRoot string) (*detect.ProjectInfo, bool) {
	slog.Debug("python detector: checking for project indicators", "path", repoRoot)

	info := &detect.ProjectInfo{
		Language: "python",
		Metadata: make(map[string]string),
	}

	hasPyproject := detect.FileExists(filepath.Join(repoRoot, "pyproject.toml"))
	hasSetupPy := detect.FileExists(filepath.Join(repoRoot, "setup.py"))
	hasSetupCfg := detect.FileExists(filepath.Join(repoRoot, "setup.cfg"))
	hasRequirements := detect.FileExists(filepath.Join(repoRoot, "requirements.txt"))
	hasPipfile := detect.FileExists(filepath.Join(repoRoot, "Pipfile"))

	if !hasPyproject && !hasSetupPy && !hasSetupCfg && !hasRequirements && !hasPipfile {
		slog.Debug("python detector: no Python project indicators found")
		return nil, false
	}

	if hasPyproject {
		d.parsePyproject(repoRoot, info)
	}
	if hasSetupPy {
		info.Metadata["setup_py"] = "true"
		slog.Debug("python detector: setup.py found (legacy)")
	}
	if hasSetupCfg {
		info.Metadata["setup_cfg"] = "true"
		slog.Debug("python detector: setup.cfg found (legacy)")
	}

	d.detectPackageManager(repoRoot, info)
	d.detectTooling(repoRoot, info)
	d.detectFramework(repoRoot, info)
	d.detectTests(repoRoot, info)
	d.detectDocs(repoRoot, info)
	d.detectLayout(repoRoot, info)

	if info.BinaryName == "" {
		info.BinaryName = filepath.Base(repoRoot)
	}
	if info.PackageManager == "" {
		info.PackageManager = "pip"
	}

	slog.Debug("python detector: detection complete",
		"name", info.BinaryName,
		"version", info.Version,
		"package_manager", info.PackageManager,
		"framework", info.Framework,
		"has_tests", info.HasTests,
	)

	return info, true
}

func (d *Detector) parsePyproject(repoRoot string, info *detect.ProjectInfo) {
	path := filepath.Join(repoRoot, "pyproject.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	slog.Debug("python detector: parsing pyproject.toml")

	content := string(data)

	if name := tomlValue(content, "[project]", "name"); name != "" {
		info.BinaryName = name
		slog.Debug("python detector: project name", "name", name)
	}

	if reqPython := tomlValue(content, "[project]", "requires-python"); reqPython != "" {
		info.Version = parsePythonVersion(reqPython)
		slog.Debug("python detector: python version", "version", info.Version)
	}

	if backend := tomlValue(content, "[build-system]", "build-backend"); backend != "" {
		info.Metadata["build_backend"] = backend
		slog.Debug("python detector: build backend", "backend", backend)
	}

	if containsSection(content, "[tool.ruff]") {
		info.Metadata["linter"] = "ruff"
		info.Metadata["formatter"] = "ruff"
	}
	if containsSection(content, "[tool.mypy]") {
		info.Metadata["typechecker"] = "mypy"
	}
	if containsSection(content, "[tool.pytest") {
		info.HasTests = true
	}
	if containsSection(content, "[tool.black]") {
		if info.Metadata["formatter"] == "" {
			info.Metadata["formatter"] = "black"
		}
	}

	if scripts := parseTomlSection(content, "[project.scripts]"); len(scripts) > 0 {
		names := make([]string, 0, len(scripts))
		for name := range scripts {
			names = append(names, name)
		}
		info.Metadata["scripts"] = strings.Join(names, ",")
		slog.Debug("python detector: project scripts", "scripts", info.Metadata["scripts"])
	}
}

func (d *Detector) detectPackageManager(repoRoot string, info *detect.ProjectInfo) {
	switch {
	case detect.FileExists(filepath.Join(repoRoot, "uv.lock")):
		info.PackageManager = "uv"
	case detect.FileExists(filepath.Join(repoRoot, "poetry.lock")):
		info.PackageManager = "poetry"
	case detect.FileExists(filepath.Join(repoRoot, "pdm.lock")):
		info.PackageManager = "pdm"
	case detect.FileExists(filepath.Join(repoRoot, "Pipfile")):
		info.PackageManager = "pipenv"
	case detect.FileExists(filepath.Join(repoRoot, "requirements.txt")):
		info.PackageManager = "pip"
	}
	slog.Debug("python detector: package manager", "manager", info.PackageManager)
}

func (d *Detector) detectTooling(repoRoot string, info *detect.ProjectInfo) {
	if info.Metadata["linter"] == "" {
		if detect.FileExists(filepath.Join(repoRoot, "ruff.toml")) || detect.FileExists(filepath.Join(repoRoot, ".ruff.toml")) {
			info.Metadata["linter"] = "ruff"
			if info.Metadata["formatter"] == "" {
				info.Metadata["formatter"] = "ruff"
			}
		} else if detect.FileExists(filepath.Join(repoRoot, ".flake8")) {
			info.Metadata["linter"] = "flake8"
		}
	}

	if info.Metadata["typechecker"] == "" {
		if detect.FileExists(filepath.Join(repoRoot, "mypy.ini")) || detect.FileExists(filepath.Join(repoRoot, ".mypy.ini")) {
			info.Metadata["typechecker"] = "mypy"
		} else if detect.FileExists(filepath.Join(repoRoot, "pyrightconfig.json")) {
			info.Metadata["typechecker"] = "pyright"
		}
	}

	if info.Metadata["formatter"] == "" {
		if detect.FileExists(filepath.Join(repoRoot, "pyproject.toml")) {
			data, err := os.ReadFile(filepath.Join(repoRoot, "pyproject.toml"))
			if err == nil && containsSection(string(data), "[tool.black]") {
				info.Metadata["formatter"] = "black"
			}
		}
	}

	slog.Debug("python detector: tooling",
		"linter", info.Metadata["linter"],
		"formatter", info.Metadata["formatter"],
		"typechecker", info.Metadata["typechecker"],
	)
}

func (d *Detector) detectFramework(repoRoot string, info *detect.ProjectInfo) {
	candidates := []string{"requirements.txt", "pyproject.toml", "setup.py", "setup.cfg"}
	var combined string
	for _, f := range candidates {
		data, err := os.ReadFile(filepath.Join(repoRoot, f))
		if err == nil {
			combined += "\n" + string(data)
		}
	}

	lower := strings.ToLower(combined)
	switch {
	case strings.Contains(lower, "django"):
		info.Framework = "django"
	case strings.Contains(lower, "fastapi"):
		info.Framework = "fastapi"
	case strings.Contains(lower, "flask"):
		info.Framework = "flask"
	}

	if info.Framework != "" {
		slog.Debug("python detector: framework detected", "framework", info.Framework)
	}
}

func (d *Detector) detectTests(repoRoot string, info *detect.ProjectInfo) {
	if info.HasTests {
		return
	}
	if detect.FileExists(filepath.Join(repoRoot, "conftest.py")) {
		info.HasTests = true
	} else if detect.FileExists(filepath.Join(repoRoot, "tests")) {
		info.HasTests = true
	} else if detect.FileExists(filepath.Join(repoRoot, "test")) {
		info.HasTests = true
	}
	slog.Debug("python detector: tests", "has_tests", info.HasTests)
}

func (d *Detector) detectDocs(repoRoot string, info *detect.ProjectInfo) {
	switch {
	case detect.FileExists(filepath.Join(repoRoot, "mkdocs.yml")) || detect.FileExists(filepath.Join(repoRoot, "mkdocs.yaml")):
		info.Metadata["docs"] = "mkdocs"
	case detect.FileExists(filepath.Join(repoRoot, "docs", "conf.py")):
		info.Metadata["docs"] = "sphinx"
	}
	if info.Metadata["docs"] != "" {
		slog.Debug("python detector: docs", "tool", info.Metadata["docs"])
	}
}

func (d *Detector) detectLayout(repoRoot string, info *detect.ProjectInfo) {
	if detect.FileExists(filepath.Join(repoRoot, "src")) {
		entries, err := os.ReadDir(filepath.Join(repoRoot, "src"))
		if err == nil {
			for _, e := range entries {
				if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && !strings.HasPrefix(e.Name(), "_") {
					info.Metadata["has_src_layout"] = "true"
					slog.Debug("python detector: src layout detected")
					return
				}
			}
		}
	}
}

// tomlValue does a best-effort extraction of a key's value from a given
// TOML section. This avoids pulling in a full TOML parser dependency for
// the limited set of fields we need.
func tomlValue(content, section, key string) string {
	idx := strings.Index(content, section)
	if idx < 0 {
		return ""
	}
	block := content[idx+len(section):]
	if nextSec := strings.Index(block, "\n["); nextSec >= 0 {
		block = block[:nextSec]
	}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key) {
			rest := strings.TrimPrefix(line, key)
			rest = strings.TrimSpace(rest)
			if len(rest) == 0 || rest[0] != '=' {
				continue
			}
			val := strings.TrimSpace(rest[1:])
			val = strings.Trim(val, "\"'")
			return val
		}
	}
	return ""
}

// parseTomlSection extracts all key = value pairs from a TOML section.
func parseTomlSection(content, section string) map[string]string {
	idx := strings.Index(content, section)
	if idx < 0 {
		return nil
	}
	block := content[idx+len(section):]
	if nextSec := strings.Index(block, "\n["); nextSec >= 0 {
		block = block[:nextSec]
	}
	result := make(map[string]string)
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eqIdx])
		val := strings.TrimSpace(line[eqIdx+1:])
		val = strings.Trim(val, "\"'")
		if key != "" {
			result[key] = val
		}
	}
	return result
}

func containsSection(content, section string) bool {
	return strings.Contains(content, section)
}

func parsePythonVersion(spec string) string {
	spec = strings.TrimSpace(spec)
	spec = strings.TrimPrefix(spec, ">=")
	spec = strings.TrimPrefix(spec, "==")
	spec = strings.TrimPrefix(spec, "~=")
	if comma := strings.Index(spec, ","); comma >= 0 {
		spec = spec[:comma]
	}
	return strings.TrimSpace(spec)
}
