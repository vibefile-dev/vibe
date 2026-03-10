package parser

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	variableRe = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*(.+)$`)
	targetRe   = regexp.MustCompile(`^([a-z][a-z0-9_-]*)\s*(?::\s*([a-z][a-z0-9_ -]*))?:$`)
)

// Parse reads a Vibefile string and returns a structured representation.
func Parse(input string) (*Vibefile, error) {
	vf := &Vibefile{
		Variables: make(map[string]string),
		Targets:   make(map[string]*Target),
	}

	lines := strings.Split(input, "\n")
	var currentTarget *Target
	inRecipe := false // tracks multi-line recipe spanning across lines

	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		line := strings.TrimRight(raw, " \t\r")

		// Inside a multi-line recipe — accumulate until closing quote.
		if inRecipe && currentTarget != nil {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.HasSuffix(trimmed, `"`) {
				currentTarget.Recipe += " " + trimmed[:len(trimmed)-1]
				inRecipe = false
			} else {
				currentTarget.Recipe += " " + trimmed
			}
			continue
		}

		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}

		isIndented := len(line) > 0 && (line[0] == ' ' || line[0] == '\t')
		trimmed := strings.TrimSpace(line)

		if isIndented && currentTarget != nil {
			opened, err := parseTargetBody(currentTarget, trimmed)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			if opened {
				inRecipe = true
			}
			continue
		}

		// Not indented — either a variable or a target header.
		currentTarget = nil

		if m := variableRe.FindStringSubmatch(trimmed); m != nil {
			vf.Variables[m[1]] = strings.TrimSpace(m[2])
			continue
		}

		if m := targetRe.FindStringSubmatch(trimmed); m != nil {
			name := m[1]
			if _, exists := vf.Targets[name]; exists {
				return nil, fmt.Errorf("line %d: duplicate target %q", i+1, name)
			}
			t := &Target{Name: name}
			if deps := strings.TrimSpace(m[2]); deps != "" {
				t.Dependencies = strings.Fields(deps)
			}
			vf.Targets[name] = t
			vf.Order = append(vf.Order, name)
			currentTarget = t
			continue
		}

		return nil, fmt.Errorf("line %d: unrecognised syntax: %s", i+1, trimmed)
	}

	if inRecipe {
		return nil, fmt.Errorf("unterminated recipe string (missing closing quote)")
	}

	if err := validate(vf); err != nil {
		return nil, err
	}

	return vf, nil
}

// parseTargetBody processes a single indented line within a target definition.
// Returns true if a multi-line recipe was opened (starts with " but doesn't end with ").
func parseTargetBody(t *Target, line string) (bool, error) {
	// Per-target model override
	if m := variableRe.FindStringSubmatch(line); m != nil && m[1] == "model" {
		t.Model = strings.TrimSpace(m[2])
		return false, nil
	}

	// Directive
	if strings.HasPrefix(line, "@") {
		return false, parseDirective(t, line)
	}

	// Recipe — double-quoted string (single or multi-line)
	if strings.HasPrefix(line, `"`) {
		content := line[1:] // strip opening quote
		if strings.HasSuffix(content, `"`) {
			// Single-line: opening and closing quote on same line
			content = content[:len(content)-1]
			if t.Recipe != "" {
				t.Recipe += " " + content
			} else {
				t.Recipe = content
			}
			return false, nil
		}
		// Multi-line: opening quote but no closing quote — continuation follows
		if t.Recipe != "" {
			t.Recipe += " " + content
		} else {
			t.Recipe = content
		}
		return true, nil
	}

	return false, fmt.Errorf("unexpected target body: %s", line)
}

func parseDirective(t *Target, line string) error {
	parts := strings.SplitN(line[1:], " ", 2)
	name := parts[0]
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	known := map[string]bool{
		"require":    true,
		"mcp":        true,
		"skill":      true,
		"network":    true,
		"no-sandbox": true,
	}
	if !known[name] {
		return fmt.Errorf("unknown directive @%s", name)
	}

	t.Directives = append(t.Directives, Directive{Name: name, Args: args})
	return nil
}

// SubstituteVars replaces $(VAR) references in a string with variable values.
func SubstituteVars(s string, vars map[string]string) string {
	result := s
	for k, v := range vars {
		result = strings.ReplaceAll(result, "$("+k+")", v)
	}
	return result
}

func validate(vf *Vibefile) error {
	for _, t := range vf.Targets {
		for _, dep := range t.Dependencies {
			if _, ok := vf.Targets[dep]; !ok {
				return fmt.Errorf("target %q depends on unknown target %q", t.Name, dep)
			}
		}
		if t.Recipe == "" && !t.HasDirective("skill") {
			return fmt.Errorf("target %q has no recipe and no @skill directive", t.Name)
		}
	}
	return nil
}
