package detect

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Template describes the variables and targets for a generated Vibefile.
type Template struct {
	Variables []TemplateVariable `yaml:"variables"`
	Targets   []TemplateTarget   `yaml:"targets"`
}

// TemplateVariable is a key-value pair for the Vibefile header.
type TemplateVariable struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

// TemplateTarget is a single target definition for the generated Vibefile.
type TemplateTarget struct {
	Name         string   `yaml:"name"`
	Dependencies []string `yaml:"dependencies,omitempty"`
	Recipe       string   `yaml:"recipe"`
	Section      string   `yaml:"section,omitempty"`
	Directives   []string `yaml:"directives,omitempty"`
}

// TemplateProvider returns a Template for a detected project. Built-in
// providers are Go structs; external providers are loaded from YAML.
type TemplateProvider interface {
	Language() string
	Provide(project *ProjectInfo) *Template
}

var providers []TemplateProvider

// RegisterTemplate adds a template provider to the registry.
func RegisterTemplate(p TemplateProvider) {
	providers = append(providers, p)
}

// ResolveTemplate finds the best template for the given language by checking:
// 1. .vibe/templates/<lang>.yaml (project-local)
// 2. ~/.vibe/templates/<lang>.yaml (user-global)
// 3. Built-in provider
func ResolveTemplate(repoRoot, language string, project *ProjectInfo) (*Template, error) {
	localPath := filepath.Join(repoRoot, ".vibe", "templates", language+".yaml")
	if t, err := loadYAMLTemplate(localPath); err == nil {
		return t, nil
	}

	if home, err := os.UserHomeDir(); err == nil {
		globalPath := filepath.Join(home, ".vibe", "templates", language+".yaml")
		if t, err := loadYAMLTemplate(globalPath); err == nil {
			return t, nil
		}
	}

	for _, p := range providers {
		if p.Language() == language {
			return p.Provide(project), nil
		}
	}

	return nil, fmt.Errorf("no template found for language %q", language)
}

func loadYAMLTemplate(path string) (*Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Template
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &t, nil
}

// ResolveSubProjectTemplate resolves a template for a SubProject. When the
// sub-project lives in a subdirectory, target names get prefixed with the
// directory name (build -> go-build), dependencies are prefixed, recipes get
// directory context, and sections include the directory name.
func ResolveSubProjectTemplate(repoRoot string, sp SubProject) (*Template, error) {
	tmpl, err := ResolveTemplate(repoRoot, sp.Project.Language, sp.Project)
	if err != nil {
		return nil, err
	}

	if sp.Dir == "" {
		return tmpl, nil
	}

	// Strip variables — the caller creates shared variables for the monorepo
	tmpl.Variables = nil

	for i := range tmpl.Targets {
		t := &tmpl.Targets[i]
		t.Name = sp.Dir + "-" + t.Name
		for j := range t.Dependencies {
			t.Dependencies[j] = sp.Dir + "-" + t.Dependencies[j]
		}
		if t.Recipe != "" {
			t.Recipe = "in the " + sp.Dir + "/ directory, " + t.Recipe
		}
		if t.Section != "" {
			t.Section = sp.Dir + " — " + t.Section
		}
	}

	return tmpl, nil
}

// MergeAddons appends addon targets into the template, skipping any target
// whose name already exists (first writer wins, with a debug log on conflict).
func MergeAddons(t *Template, results []*AddonResult) {
	existing := make(map[string]bool, len(t.Targets))
	for _, tgt := range t.Targets {
		existing[tgt.Name] = true
	}

	for _, r := range results {
		for _, tgt := range r.Targets {
			if existing[tgt.Name] {
				slog.Debug("addon target skipped (name conflict)",
					"addon", r.Label, "target", tgt.Name)
				continue
			}
			existing[tgt.Name] = true
			t.Targets = append(t.Targets, tgt)
		}
	}
}

// Generate renders a Template to Vibefile text.
func Generate(t *Template) string {
	var b strings.Builder

	if len(t.Variables) > 0 {
		maxKeyLen := 0
		for _, v := range t.Variables {
			if len(v.Key) > maxKeyLen {
				maxKeyLen = len(v.Key)
			}
		}
		for _, v := range t.Variables {
			padding := strings.Repeat(" ", maxKeyLen-len(v.Key))
			fmt.Fprintf(&b, "%s%s = %s\n", v.Key, padding, v.Value)
		}
		b.WriteString("\n")
	}

	currentSection := ""
	for _, target := range t.Targets {
		if target.Section != "" && target.Section != currentSection {
			if currentSection != "" {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "# ── %s %s\n\n", target.Section, sectionPadding(target.Section))
			currentSection = target.Section
		}

		if len(target.Dependencies) > 0 {
			fmt.Fprintf(&b, "%s: %s:\n", target.Name, strings.Join(target.Dependencies, " "))
		} else {
			fmt.Fprintf(&b, "%s:\n", target.Name)
		}

		if target.Recipe != "" {
			fmt.Fprintf(&b, "    \"%s\"\n", target.Recipe)
		}

		for _, d := range target.Directives {
			fmt.Fprintf(&b, "    %s\n", d)
		}

		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

func sectionPadding(section string) string {
	total := 52
	used := len("# ── ") + len(section) + 1
	if used >= total {
		return "──"
	}
	return strings.Repeat("─", total-used)
}
