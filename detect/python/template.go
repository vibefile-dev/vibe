package python

import (
	"fmt"

	"github.com/vibefile-dev/vibe/detect"
)

func init() { detect.RegisterTemplate(&TemplateProvider{}) }

// TemplateProvider generates a Vibefile template for Python projects.
type TemplateProvider struct{}

func (p *TemplateProvider) Language() string { return "python" }

func (p *TemplateProvider) Provide(project *detect.ProjectInfo) *detect.Template {
	name := project.BinaryName
	if name == "" {
		name = "app"
	}
	runner := runPrefix(project.PackageManager)

	t := &detect.Template{
		Variables: []detect.TemplateVariable{
			{Key: "model", Value: "claude-sonnet-4-6"},
			{Key: "name", Value: name},
		},
	}

	// run — only for web frameworks that have a dev server
	if project.Framework != "" {
		t.Targets = append(t.Targets, detect.TemplateTarget{
			Name:    "run",
			Section: "development",
			Recipe:  runRecipe(project.Framework, runner),
		})
	}

	// test
	t.Targets = append(t.Targets, detect.TemplateTarget{
		Name:    "test",
		Section: "build & test",
		Recipe:  fmt.Sprintf("run all Python tests with pytest and verbose output%s", runner),
	})

	// lint
	linter := project.Metadata["linter"]
	switch linter {
	case "flake8":
		t.Targets = append(t.Targets, detect.TemplateTarget{
			Name:    "lint",
			Section: "build & test",
			Recipe:  fmt.Sprintf("run flake8 on the entire project%s", runner),
		})
	default:
		t.Targets = append(t.Targets, detect.TemplateTarget{
			Name:    "lint",
			Section: "build & test",
			Recipe:  fmt.Sprintf("run ruff check on the entire project%s", runner),
		})
	}

	// fmt
	formatter := project.Metadata["formatter"]
	switch formatter {
	case "black":
		t.Targets = append(t.Targets, detect.TemplateTarget{
			Name:    "fmt",
			Section: "build & test",
			Recipe:  fmt.Sprintf("format all Python files using black%s", runner),
		})
	default:
		t.Targets = append(t.Targets, detect.TemplateTarget{
			Name:    "fmt",
			Section: "build & test",
			Recipe:  fmt.Sprintf("format all Python files using ruff format%s", runner),
		})
	}

	// typecheck — only if detected
	if tc := project.Metadata["typechecker"]; tc != "" {
		t.Targets = append(t.Targets, detect.TemplateTarget{
			Name:    "typecheck",
			Section: "build & test",
			Recipe:  fmt.Sprintf("run %s on the project%s", tc, runner),
		})
	}

	// build — only if pyproject.toml has a build backend
	if project.Metadata["build_backend"] != "" {
		t.Targets = append(t.Targets, detect.TemplateTarget{
			Name:    "build",
			Section: "build & test",
			Recipe:  fmt.Sprintf("build the Python package%s", runner),
		})
	}

	// install
	t.Targets = append(t.Targets, detect.TemplateTarget{
		Name:    "install",
		Section: "housekeeping",
		Recipe:  installRecipe(project.PackageManager),
	})

	// clean
	t.Targets = append(t.Targets, detect.TemplateTarget{
		Name:    "clean",
		Section: "housekeeping",
		Recipe:  "remove __pycache__, .pytest_cache, dist/, build/, *.egg-info, and .coverage",
	})

	// check — depends on fmt, lint, typecheck (if present), test
	checkDeps := []string{"fmt", "lint"}
	if project.Metadata["typechecker"] != "" {
		checkDeps = append(checkDeps, "typecheck")
	}
	checkDeps = append(checkDeps, "test")

	t.Targets = append(t.Targets, detect.TemplateTarget{
		Name:         "check",
		Section:      "quality gates",
		Dependencies: checkDeps,
		Recipe:       "echo all quality gates passed",
	})

	return t
}

func runRecipe(framework, runner string) string {
	switch framework {
	case "fastapi":
		return fmt.Sprintf("start the FastAPI development server with auto-reload%s", runner)
	case "flask":
		return fmt.Sprintf("start the Flask development server with debug mode%s", runner)
	case "django":
		return fmt.Sprintf("start the Django development server%s", runner)
	default:
		return fmt.Sprintf("start the application%s", runner)
	}
}

func installRecipe(pm string) string {
	switch pm {
	case "uv":
		return "install all dependencies using uv sync --frozen --all-extras, then run pre-commit install --install-hooks if pre-commit is available"
	case "poetry":
		return "install all dependencies using poetry install --no-interaction, then run pre-commit install --install-hooks if pre-commit is available"
	case "pdm":
		return "install all dependencies using pdm install, then run pre-commit install --install-hooks if pre-commit is available"
	case "pipenv":
		return "install all dependencies using pipenv install --dev, then run pre-commit install --install-hooks if pre-commit is available"
	default:
		return "install all dependencies using pip install -r requirements.txt, then run pre-commit install --install-hooks if pre-commit is available"
	}
}

func runPrefix(pm string) string {
	switch pm {
	case "uv":
		return " using uv run"
	case "poetry":
		return " using poetry run"
	case "pdm":
		return " using pdm run"
	case "pipenv":
		return " using pipenv run"
	default:
		return ""
	}
}
