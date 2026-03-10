package resolver

import (
	"testing"

	"github.com/vibefile-dev/vibe/parser"
)

func vibefile(targets map[string]*parser.Target) *parser.Vibefile {
	return &parser.Vibefile{Targets: targets}
}

func target(name string, deps ...string) *parser.Target {
	return &parser.Target{Name: name, Dependencies: deps, Recipe: "do " + name}
}

func TestResolveSingleTarget(t *testing.T) {
	vf := vibefile(map[string]*parser.Target{
		"build": target("build"),
	})
	order, err := Resolve(vf, "build")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 1 || order[0] != "build" {
		t.Errorf("expected [build], got %v", order)
	}
}

func TestResolveLinearChain(t *testing.T) {
	vf := vibefile(map[string]*parser.Target{
		"test":   target("test"),
		"build":  target("build", "test"),
		"deploy": target("deploy", "build"),
	})
	order, err := Resolve(vf, "deploy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"test", "build", "deploy"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d targets, got %d: %v", len(expected), len(order), order)
	}
	for i, name := range expected {
		if order[i] != name {
			t.Errorf("order[%d]: expected %q, got %q", i, name, order[i])
		}
	}
}

func TestResolveDiamond(t *testing.T) {
	vf := vibefile(map[string]*parser.Target{
		"a": target("a"),
		"b": target("b", "a"),
		"c": target("c", "a"),
		"d": target("d", "b", "c"),
	})
	order, err := Resolve(vf, "d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pos := make(map[string]int)
	for i, name := range order {
		pos[name] = i
	}
	if pos["a"] >= pos["b"] || pos["a"] >= pos["c"] {
		t.Errorf("a must come before b and c: %v", order)
	}
	if pos["b"] >= pos["d"] || pos["c"] >= pos["d"] {
		t.Errorf("b and c must come before d: %v", order)
	}
}

func TestResolveCycleError(t *testing.T) {
	vf := vibefile(map[string]*parser.Target{
		"a": target("a", "b"),
		"b": target("b", "a"),
	})
	_, err := Resolve(vf, "a")
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestResolveUnknownTarget(t *testing.T) {
	vf := vibefile(map[string]*parser.Target{
		"build": target("build"),
	})
	_, err := Resolve(vf, "deploy")
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestResolveUnknownDependency(t *testing.T) {
	vf := vibefile(map[string]*parser.Target{
		"deploy": target("deploy", "missing"),
	})
	_, err := Resolve(vf, "deploy")
	if err == nil {
		t.Fatal("expected error for unknown dependency")
	}
}

func TestFormatChain(t *testing.T) {
	result := FormatChain([]string{"test", "build", "deploy"})
	expected := "test → build → deploy"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFormatChainSingle(t *testing.T) {
	result := FormatChain([]string{"build"})
	if result != "build" {
		t.Errorf("expected %q, got %q", "build", result)
	}
}
