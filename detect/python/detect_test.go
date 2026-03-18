package python

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vibefile-dev/vibe/detect"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetect_PyprojectUV(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
name = "my-api"
requires-python = ">=3.12"

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[tool.ruff]
line-length = 88

[tool.mypy]
strict = true
`)
	writeFile(t, dir, "uv.lock", "")
	writeFile(t, dir, "tests/test_main.py", "")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "Language", info.Language, "python")
	assertField(t, "PackageManager", info.PackageManager, "uv")
	assertField(t, "BinaryName", info.BinaryName, "my-api")
	assertField(t, "Version", info.Version, "3.12")
	assertField(t, "linter", info.Metadata["linter"], "ruff")
	assertField(t, "typechecker", info.Metadata["typechecker"], "mypy")
	assertField(t, "formatter", info.Metadata["formatter"], "ruff")
	assertField(t, "build_backend", info.Metadata["build_backend"], "hatchling.build")
	if !info.HasTests {
		t.Error("expected HasTests = true")
	}
}

func TestDetect_PyprojectPoetry(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
name = "poetry-app"
requires-python = ">=3.11"

[build-system]
requires = ["poetry-core"]
build-backend = "poetry.core.masonry.api"

[tool.black]
line-length = 120
`)
	writeFile(t, dir, "poetry.lock", "")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "Language", info.Language, "python")
	assertField(t, "PackageManager", info.PackageManager, "poetry")
	assertField(t, "BinaryName", info.BinaryName, "poetry-app")
	assertField(t, "Version", info.Version, "3.11")
	assertField(t, "formatter", info.Metadata["formatter"], "black")
	assertField(t, "build_backend", info.Metadata["build_backend"], "poetry.core.masonry.api")
}

func TestDetect_RequirementsOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "flask==3.0.0\nrequests\n")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "Language", info.Language, "python")
	assertField(t, "PackageManager", info.PackageManager, "pip")
	assertField(t, "Framework", info.Framework, "flask")
	assertField(t, "BinaryName", info.BinaryName, filepath.Base(dir))
}

func TestDetect_SetupPyLegacy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "setup.py", `
from setuptools import setup
setup(name="legacy-pkg", version="1.0")
`)
	writeFile(t, dir, "requirements.txt", "")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "Language", info.Language, "python")
	assertField(t, "setup_py", info.Metadata["setup_py"], "true")
}

func TestDetect_WithRuffStandalone(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
name = "ruff-proj"
`)
	writeFile(t, dir, "ruff.toml", "[lint]\nselect = [\"E\", \"F\"]\n")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "linter", info.Metadata["linter"], "ruff")
	assertField(t, "formatter", info.Metadata["formatter"], "ruff")
}

func TestDetect_WithFlake8(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
name = "flake8-proj"
`)
	writeFile(t, dir, ".flake8", "[flake8]\nmax-line-length = 120\n")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "linter", info.Metadata["linter"], "flake8")
}

func TestDetect_WithMypy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
name = "mypy-proj"
`)
	writeFile(t, dir, "mypy.ini", "[mypy]\nstrict = True\n")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "typechecker", info.Metadata["typechecker"], "mypy")
}

func TestDetect_WithPyright(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
name = "pyright-proj"
`)
	writeFile(t, dir, "pyrightconfig.json", "{}")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "typechecker", info.Metadata["typechecker"], "pyright")
}

func TestDetect_WithMkdocs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
name = "docs-proj"
`)
	writeFile(t, dir, "mkdocs.yml", "site_name: docs\n")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "docs", info.Metadata["docs"], "mkdocs")
}

func TestDetect_WithSphinx(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
name = "sphinx-proj"
`)
	writeFile(t, dir, "docs/conf.py", "project = 'sphinx-proj'\n")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "docs", info.Metadata["docs"], "sphinx")
}

func TestDetect_SrcLayout(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
name = "src-proj"
`)
	writeFile(t, dir, "src/mypackage/__init__.py", "")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "has_src_layout", info.Metadata["has_src_layout"], "true")
}

func TestDetect_FastAPI(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "fastapi>=0.100\nuvicorn\n")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "Framework", info.Framework, "fastapi")
}

func TestDetect_Django(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "django>=5.0\n")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "Framework", info.Framework, "django")
}

func TestDetect_PDM(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
name = "pdm-app"
`)
	writeFile(t, dir, "pdm.lock", "")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "PackageManager", info.PackageManager, "pdm")
}

func TestDetect_Pipenv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Pipfile", "[packages]\n")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "PackageManager", info.PackageManager, "pipenv")
}

func TestDetect_Scripts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
name = "mycli"

[project.scripts]
mycli = "mycli.main:main"
`)

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	assertField(t, "scripts", info.Metadata["scripts"], "mycli")
}

func TestTemplate_RunWithFramework(t *testing.T) {
	project := &detect.ProjectInfo{
		Language:       "python",
		Framework:      "fastapi",
		PackageManager: "uv",
		BinaryName:     "myapi",
		Metadata:       map[string]string{},
	}

	p := &TemplateProvider{}
	tmpl := p.Provide(project)
	targets := targetMap(tmpl)

	assertContains(t, "run recipe", targets["run"], "FastAPI")
	assertContains(t, "run recipe", targets["run"], "using uv run")
}

func TestTemplate_NoRunForCLITool(t *testing.T) {
	project := &detect.ProjectInfo{
		Language:       "python",
		PackageManager: "uv",
		BinaryName:     "mycli",
		Metadata: map[string]string{
			"scripts": "mycli",
		},
	}

	p := &TemplateProvider{}
	tmpl := p.Provide(project)

	for _, tgt := range tmpl.Targets {
		if tgt.Name == "run" {
			t.Error("run target should not exist for a CLI tool without a web framework")
		}
	}
}

func TestDetect_NoIndicators(t *testing.T) {
	dir := t.TempDir()
	d := &Detector{}
	_, ok := d.Detect(dir)
	if ok {
		t.Fatal("expected detection to fail for empty directory")
	}
}

func TestDetect_Conftest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
name = "conftest-proj"
`)
	writeFile(t, dir, "conftest.py", "")

	d := &Detector{}
	info, ok := d.Detect(dir)
	if !ok {
		t.Fatal("expected detection to succeed")
	}

	if !info.HasTests {
		t.Error("expected HasTests = true when conftest.py exists")
	}
}

// --- template tests ---

func TestTemplate_UV(t *testing.T) {
	project := &detect.ProjectInfo{
		Language:       "python",
		PackageManager: "uv",
		BinaryName:     "my-app",
		Metadata: map[string]string{
			"linter":        "ruff",
			"formatter":     "ruff",
			"typechecker":   "mypy",
			"build_backend": "hatchling.build",
		},
	}

	p := &TemplateProvider{}
	tmpl := p.Provide(project)

	targets := targetMap(tmpl)

	assertContains(t, "test recipe", targets["test"], "pytest")
	assertContains(t, "test recipe", targets["test"], "using uv run")
	assertContains(t, "lint recipe", targets["lint"], "ruff check")
	assertContains(t, "lint recipe", targets["lint"], "using uv run")
	assertContains(t, "fmt recipe", targets["fmt"], "ruff format")
	assertContains(t, "typecheck recipe", targets["typecheck"], "mypy")
	assertTargetExists(t, tmpl, "build")
	assertTargetExists(t, tmpl, "install")
	assertContains(t, "install recipe", targets["install"], "uv sync")
	assertContains(t, "install recipe", targets["install"], "pre-commit")
	assertTargetExists(t, tmpl, "clean")
	assertTargetExists(t, tmpl, "check")
}

func TestTemplate_Poetry(t *testing.T) {
	project := &detect.ProjectInfo{
		Language:       "python",
		PackageManager: "poetry",
		BinaryName:     "poetry-app",
		Metadata: map[string]string{
			"formatter": "black",
		},
	}

	p := &TemplateProvider{}
	tmpl := p.Provide(project)

	targets := targetMap(tmpl)

	assertContains(t, "test recipe", targets["test"], "using poetry run")
	assertContains(t, "fmt recipe", targets["fmt"], "black")
	assertContains(t, "fmt recipe", targets["fmt"], "using poetry run")
}

func TestTemplate_Pip(t *testing.T) {
	project := &detect.ProjectInfo{
		Language:       "python",
		PackageManager: "pip",
		BinaryName:     "pip-app",
		Metadata:       map[string]string{},
	}

	p := &TemplateProvider{}
	tmpl := p.Provide(project)

	targets := targetMap(tmpl)

	assertNotContains(t, "test recipe", targets["test"], "using")
}

func TestTemplate_NoTypecheck(t *testing.T) {
	project := &detect.ProjectInfo{
		Language:       "python",
		PackageManager: "pip",
		BinaryName:     "simple",
		Metadata:       map[string]string{},
	}

	p := &TemplateProvider{}
	tmpl := p.Provide(project)

	for _, tgt := range tmpl.Targets {
		if tgt.Name == "typecheck" {
			t.Error("typecheck target should not exist when no type checker detected")
		}
	}
}

func TestTemplate_NoBuild(t *testing.T) {
	project := &detect.ProjectInfo{
		Language:       "python",
		PackageManager: "pip",
		BinaryName:     "simple",
		Metadata:       map[string]string{},
	}

	p := &TemplateProvider{}
	tmpl := p.Provide(project)

	for _, tgt := range tmpl.Targets {
		if tgt.Name == "build" {
			t.Error("build target should not exist without a build backend")
		}
	}
}

func TestTemplate_CheckDepsIncludeTypecheck(t *testing.T) {
	project := &detect.ProjectInfo{
		Language:       "python",
		PackageManager: "pip",
		BinaryName:     "tc-proj",
		Metadata: map[string]string{
			"typechecker": "pyright",
		},
	}

	p := &TemplateProvider{}
	tmpl := p.Provide(project)

	for _, tgt := range tmpl.Targets {
		if tgt.Name == "check" {
			found := false
			for _, dep := range tgt.Dependencies {
				if dep == "typecheck" {
					found = true
				}
			}
			if !found {
				t.Error("check target should depend on typecheck when type checker is detected")
			}
			return
		}
	}
	t.Error("check target not found")
}

func TestTemplate_Flake8(t *testing.T) {
	project := &detect.ProjectInfo{
		Language:       "python",
		PackageManager: "pip",
		BinaryName:     "flake8-proj",
		Metadata: map[string]string{
			"linter": "flake8",
		},
	}

	p := &TemplateProvider{}
	tmpl := p.Provide(project)

	targets := targetMap(tmpl)
	assertContains(t, "lint recipe", targets["lint"], "flake8")
}

// --- helpers ---

func assertField(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", name, got, want)
	}
}

func assertContains(t *testing.T, label, haystack, needle string) {
	t.Helper()
	if haystack == "" {
		t.Errorf("%s: target not found", label)
		return
	}
	if !contains(haystack, needle) {
		t.Errorf("%s: %q does not contain %q", label, haystack, needle)
	}
}

func assertNotContains(t *testing.T, label, haystack, needle string) {
	t.Helper()
	if contains(haystack, needle) {
		t.Errorf("%s: %q should not contain %q", label, haystack, needle)
	}
}

func assertTargetExists(t *testing.T, tmpl *detect.Template, name string) {
	t.Helper()
	for _, tgt := range tmpl.Targets {
		if tgt.Name == name {
			return
		}
	}
	t.Errorf("target %q not found in template", name)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func targetMap(tmpl *detect.Template) map[string]string {
	m := make(map[string]string)
	for _, tgt := range tmpl.Targets {
		m[tgt.Name] = tgt.Recipe
	}
	return m
}
