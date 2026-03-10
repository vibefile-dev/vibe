package golang

import (
	"fmt"
	"strings"

	"github.com/vibefiledev/vibe/detect"
)

func init() { detect.RegisterTemplate(&TemplateProvider{}) }

// TemplateProvider generates a Vibefile template for Go projects.
type TemplateProvider struct{}

func (p *TemplateProvider) Language() string { return "go" }

func (p *TemplateProvider) Provide(project *detect.ProjectInfo) *detect.Template {
	if len(project.Modules) > 0 {
		return p.provideWorkspace(project)
	}
	return p.provideSingleModule(project)
}

func (p *TemplateProvider) provideSingleModule(project *detect.ProjectInfo) *detect.Template {
	name := project.BinaryName
	if name == "" {
		name = "app"
	}

	t := &detect.Template{
		Variables: []detect.TemplateVariable{
			{Key: "model", Value: "claude-sonnet-4-6"},
			{Key: "name", Value: name},
		},
	}

	if project.Module != "" {
		t.Variables = append(t.Variables, detect.TemplateVariable{Key: "module", Value: project.Module})
	}

	t.Targets = append(t.Targets,
		detect.TemplateTarget{
			Name:    "build",
			Section: "build & test",
			Recipe:  "compile the Go binary named $(name) from the module root",
		},
		detect.TemplateTarget{
			Name:    "test",
			Section: "build & test",
			Recipe:  "run all Go tests in every package with verbose output and race detection enabled",
		},
		detect.TemplateTarget{
			Name:    "lint",
			Section: "build & test",
			Recipe:  "run golangci-lint on the entire module. if golangci-lint is not installed, install it first using go install",
		},
		detect.TemplateTarget{
			Name:    "fmt",
			Section: "build & test",
			Recipe:  "format all .go files using gofmt -s -w and then run go mod tidy",
		},
		detect.TemplateTarget{
			Name:    "vet",
			Section: "build & test",
			Recipe:  "run go vet on all packages in the module",
		},
		detect.TemplateTarget{
			Name:         "check",
			Section:      "quality gates",
			Dependencies: []string{"fmt", "vet", "lint", "test"},
			Recipe:       "print a summary of what passed — all quality gates complete",
		},
		detect.TemplateTarget{
			Name:    "clean",
			Section: "housekeeping",
			Recipe:  "remove the $(name) binary and the dist/ directory if they exist",
		},
		detect.TemplateTarget{
			Name:         "install",
			Section:      "housekeeping",
			Dependencies: []string{"build"},
			Recipe:       "move the compiled $(name) binary to $GOPATH/bin or ~/go/bin",
		},
	)

	return t
}

func (p *TemplateProvider) provideWorkspace(project *detect.ProjectInfo) *detect.Template {
	name := project.BinaryName
	if name == "" {
		name = "app"
	}

	moduleDirs := strings.Join(project.Modules, ", ")
	modulePackages := make([]string, len(project.Modules))
	for i, m := range project.Modules {
		modulePackages[i] = m + "/..."
	}
	allPackages := strings.Join(modulePackages, " ")

	t := &detect.Template{
		Variables: []detect.TemplateVariable{
			{Key: "model", Value: "claude-sonnet-4-6"},
			{Key: "name", Value: name},
		},
	}

	if project.Module != "" {
		t.Variables = append(t.Variables, detect.TemplateVariable{Key: "module", Value: project.Module})
	}

	t.Targets = append(t.Targets,
		detect.TemplateTarget{
			Name:    "build",
			Section: "build & test",
			Recipe:  fmt.Sprintf("compile all Go packages across workspace modules (%s) — this is a go.work workspace, use go build for each module", moduleDirs),
		},
		detect.TemplateTarget{
			Name:    "test",
			Section: "build & test",
			Recipe:  fmt.Sprintf("run all Go tests with verbose output and race detection across workspace packages: %s", allPackages),
		},
		detect.TemplateTarget{
			Name:    "lint",
			Section: "build & test",
			Recipe:  fmt.Sprintf("run golangci-lint on each workspace module directory (%s). if golangci-lint is not installed, install it first using go install", moduleDirs),
		},
		detect.TemplateTarget{
			Name:    "fmt",
			Section: "build & test",
			Recipe:  fmt.Sprintf("format all .go files using gofmt -s -w across workspace modules (%s) and then run go mod tidy in each module directory", moduleDirs),
		},
		detect.TemplateTarget{
			Name:    "vet",
			Section: "build & test",
			Recipe:  fmt.Sprintf("run go vet on all packages across workspace modules: %s", allPackages),
		},
		detect.TemplateTarget{
			Name:         "check",
			Section:      "quality gates",
			Dependencies: []string{"fmt", "vet", "lint", "test"},
			Recipe:       "print a summary of what passed — all quality gates complete",
		},
		detect.TemplateTarget{
			Name:    "clean",
			Section: "housekeeping",
			Recipe:  fmt.Sprintf("remove build artifacts and run go clean across workspace modules (%s)", moduleDirs),
		},
	)

	return t
}
