package nextjs

import (
	"fmt"

	"github.com/vibefiledev/vibe/detect"
)

func init() { detect.RegisterTemplate(&TemplateProvider{}) }

// TemplateProvider generates a Vibefile template for Next.js projects.
type TemplateProvider struct{}

func (p *TemplateProvider) Language() string { return "nextjs" }

func (p *TemplateProvider) Provide(project *detect.ProjectInfo) *detect.Template {
	name := project.BinaryName
	if name == "" {
		name = "app"
	}
	pm := project.PackageManager
	runner := runCmd(pm)

	t := &detect.Template{
		Variables: []detect.TemplateVariable{
			{Key: "model", Value: "claude-sonnet-4-6"},
			{Key: "name", Value: name},
		},
	}

	t.Targets = append(t.Targets, detect.TemplateTarget{
		Name:    "dev",
		Section: "development",
		Recipe:  fmt.Sprintf("start the Next.js development server using %s", runner),
	})

	t.Targets = append(t.Targets,
		detect.TemplateTarget{
			Name:    "build",
			Section: "build & test",
			Recipe:  fmt.Sprintf("build the Next.js application for production using %s", runner),
		},
		detect.TemplateTarget{
			Name:    "start",
			Section: "build & test",
			Recipe:  fmt.Sprintf("start the Next.js production server using %s", runner),
		},
		detect.TemplateTarget{
			Name:    "lint",
			Section: "build & test",
			Recipe:  fmt.Sprintf("run ESLint on the entire project using %s", runner),
		},
	)

	if project.Metadata["typescript"] == "true" {
		t.Targets = append(t.Targets, detect.TemplateTarget{
			Name:    "typecheck",
			Section: "build & test",
			Recipe:  fmt.Sprintf("run TypeScript type checking with tsc --noEmit using %s", runner),
		})
	}

	if project.Metadata["prettier"] == "true" {
		t.Targets = append(t.Targets, detect.TemplateTarget{
			Name:    "format",
			Section: "build & test",
			Recipe:  fmt.Sprintf("format all source files using Prettier via %s, writing changes in place", runner),
		})
	}

	if testFw := project.Metadata["test_framework"]; testFw != "" {
		t.Targets = append(t.Targets, detect.TemplateTarget{
			Name:    "test",
			Section: "build & test",
			Recipe:  fmt.Sprintf("run all unit tests using %s via %s", testFw, runner),
		})
	} else if project.Metadata["has_test_script"] == "true" {
		t.Targets = append(t.Targets, detect.TemplateTarget{
			Name:    "test",
			Section: "build & test",
			Recipe:  fmt.Sprintf("run the project's test script using %s", runner),
		})
	}

	if e2e := project.Metadata["e2e_framework"]; e2e != "" {
		t.Targets = append(t.Targets, detect.TemplateTarget{
			Name:    "e2e",
			Section: "build & test",
			Recipe:  fmt.Sprintf("run end-to-end tests using %s via %s", e2e, runner),
		})
	}

	checkDeps := []string{"lint"}
	if project.Metadata["typescript"] == "true" {
		checkDeps = append(checkDeps, "typecheck")
	}
	if project.Metadata["prettier"] == "true" {
		checkDeps = append(checkDeps, "format")
	}
	if project.Metadata["test_framework"] != "" || project.Metadata["has_test_script"] == "true" {
		checkDeps = append(checkDeps, "test")
	}
	checkDeps = append(checkDeps, "build")

	t.Targets = append(t.Targets, detect.TemplateTarget{
		Name:         "check",
		Section:      "quality gates",
		Dependencies: checkDeps,
		Recipe:       "print a summary of what passed — all quality gates complete",
	})

	t.Targets = append(t.Targets,
		detect.TemplateTarget{
			Name:    "clean",
			Section: "housekeeping",
			Recipe:  "remove .next/, node_modules/, and out/ directories if they exist",
		},
		detect.TemplateTarget{
			Name:    "deps",
			Section: "housekeeping",
			Recipe:  fmt.Sprintf("install all dependencies using %s", pm),
		},
	)

	return t
}

func runCmd(pm string) string {
	switch pm {
	case "pnpm":
		return "pnpm"
	case "yarn":
		return "yarn"
	case "bun":
		return "bun"
	default:
		return "npm run"
	}
}
