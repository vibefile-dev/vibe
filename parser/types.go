package parser

// Vibefile represents a parsed Vibefile.
type Vibefile struct {
	Variables map[string]string
	Targets   map[string]*Target
	Order     []string // target names in declaration order
}

// Target represents a single target definition in a Vibefile.
type Target struct {
	Name         string
	Dependencies []string
	Recipe       string
	Model        string // per-target model override
	Directives   []Directive
}

// Directive represents a modifier on a target (e.g. @require, @mcp, @skill, @network, @no-sandbox).
type Directive struct {
	Name string // "require", "mcp", "skill", "network", "no-sandbox"
	Args string
}

// ExecutionMode returns the execution mode for this target based on its directives.
func (t *Target) ExecutionMode() string {
	for _, d := range t.Directives {
		if d.Name == "mcp" {
			return "agent"
		}
	}
	for _, d := range t.Directives {
		if d.Name == "skill" {
			return "skill"
		}
	}
	return "codegen"
}

// HasDirective checks whether the target has a directive with the given name.
func (t *Target) HasDirective(name string) bool {
	for _, d := range t.Directives {
		if d.Name == name {
			return true
		}
	}
	return false
}

// DirectiveArgs returns the args for the first directive with the given name, or empty string.
func (t *Target) DirectiveArgs(name string) string {
	for _, d := range t.Directives {
		if d.Name == name {
			return d.Args
		}
	}
	return ""
}
