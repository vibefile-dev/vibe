package resolver

import (
	"fmt"
	"strings"

	"github.com/vibefile-dev/vibe/parser"
)

// Resolve returns targets in topological execution order for the given root target.
// Dependencies are listed before their dependents. Returns an error on cycles or
// unknown targets.
func Resolve(vf *parser.Vibefile, root string) ([]string, error) {
	target, ok := vf.Targets[root]
	if !ok {
		return nil, fmt.Errorf("unknown target %q", root)
	}

	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	var order []string

	var visit func(name string) error
	visit = func(name string) error {
		if inStack[name] {
			return fmt.Errorf("dependency cycle detected involving %q", name)
		}
		if visited[name] {
			return nil
		}
		inStack[name] = true

		t, ok := vf.Targets[name]
		if !ok {
			return fmt.Errorf("unknown target %q", name)
		}
		for _, dep := range t.Dependencies {
			if err := visit(dep); err != nil {
				return err
			}
		}

		inStack[name] = false
		visited[name] = true
		order = append(order, name)
		return nil
	}

	_ = target // already verified above
	if err := visit(root); err != nil {
		return nil, err
	}

	return order, nil
}

// FormatChain returns a human-readable representation of the execution order.
func FormatChain(order []string) string {
	return strings.Join(order, " → ")
}
