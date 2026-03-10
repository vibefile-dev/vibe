package compiled

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vibefile-dev/vibe/parser"
)

func TestPaths(t *testing.T) {
	root := "/repo"
	if got := ScriptPath(root, "build"); got != filepath.Join(root, ".vibe", "compiled", "build.sh") {
		t.Errorf("ScriptPath = %q", got)
	}
	if got := LockPath(root, "build"); got != filepath.Join(root, ".vibe", "compiled", "build.lock") {
		t.Errorf("LockPath = %q", got)
	}
}

func TestCacheDir(t *testing.T) {
	tmp := t.TempDir()
	dir, err := CacheDir(tmp)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	expected := filepath.Join(tmp, ".vibe", "compiled")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("cache dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	target := &parser.Target{Name: "build", Recipe: "compile the project"}
	vars := map[string]string{"env": "prod"}
	script := "#!/bin/bash\ngo build ."

	lock := BuildLock(target, "claude-sonnet-4-6", vars, nil, script)
	if err := Save(tmp, "build", script, lock); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, ok := Load(tmp, "build")
	if !ok {
		t.Fatal("expected script to be loadable")
	}
	if loaded != script {
		t.Errorf("loaded script = %q, want %q", loaded, script)
	}

	loadedLock, err := LoadLock(tmp, "build")
	if err != nil {
		t.Fatalf("LoadLock: %v", err)
	}
	if loadedLock.Recipe != target.Recipe {
		t.Errorf("lock recipe = %q, want %q", loadedLock.Recipe, target.Recipe)
	}
	if loadedLock.Model != "claude-sonnet-4-6" {
		t.Errorf("lock model = %q", loadedLock.Model)
	}
}

func TestLoadMissing(t *testing.T) {
	tmp := t.TempDir()
	_, ok := Load(tmp, "nonexistent")
	if ok {
		t.Error("expected ok=false for missing script")
	}
}

func TestIsValidUnchanged(t *testing.T) {
	target := &parser.Target{Name: "build", Recipe: "compile"}
	vars := map[string]string{"env": "prod"}
	ctx := map[string]string{"go.mod": "module example"}
	script := "#!/bin/bash\ngo build ."

	lock := BuildLock(target, "claude-sonnet-4-6", vars, ctx, script)

	valid, reason := IsValid(lock, target, "claude-sonnet-4-6", vars, ctx)
	if !valid {
		t.Errorf("expected valid, got invalid: %s", reason)
	}
}

func TestIsValidRecipeChanged(t *testing.T) {
	target := &parser.Target{Name: "build", Recipe: "compile"}
	lock := BuildLock(target, "claude-sonnet-4-6", nil, nil, "script")

	target.Recipe = "compile and test"
	valid, reason := IsValid(lock, target, "claude-sonnet-4-6", nil, nil)
	if valid {
		t.Error("expected invalid when recipe changed")
	}
	if reason != "recipe changed" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestIsValidModelChanged(t *testing.T) {
	target := &parser.Target{Name: "build", Recipe: "compile"}
	lock := BuildLock(target, "claude-sonnet-4-6", nil, nil, "script")

	valid, reason := IsValid(lock, target, "gpt-4o", nil, nil)
	if valid {
		t.Error("expected invalid when model changed")
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestIsValidVariablesChanged(t *testing.T) {
	target := &parser.Target{Name: "build", Recipe: "compile"}
	vars := map[string]string{"env": "prod"}
	lock := BuildLock(target, "model", vars, nil, "script")

	newVars := map[string]string{"env": "staging"}
	valid, _ := IsValid(lock, target, "model", newVars, nil)
	if valid {
		t.Error("expected invalid when variables changed")
	}
}

func TestIsValidContextFileAdded(t *testing.T) {
	target := &parser.Target{Name: "build", Recipe: "compile"}
	lock := BuildLock(target, "model", nil, nil, "script")

	newCtx := map[string]string{"go.mod": "module new"}
	valid, reason := IsValid(lock, target, "model", nil, newCtx)
	if valid {
		t.Error("expected invalid when context file added")
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestIsValidContextFileRemoved(t *testing.T) {
	target := &parser.Target{Name: "build", Recipe: "compile"}
	ctx := map[string]string{"go.mod": "module old"}
	lock := BuildLock(target, "model", nil, ctx, "script")

	valid, reason := IsValid(lock, target, "model", nil, nil)
	if valid {
		t.Error("expected invalid when context file removed")
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestIsHandEdited(t *testing.T) {
	tmp := t.TempDir()
	target := &parser.Target{Name: "build", Recipe: "compile"}
	script := "#!/bin/bash\ngo build ."
	lock := BuildLock(target, "model", nil, nil, script)

	if err := Save(tmp, "build", script, lock); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if IsHandEdited(tmp, "build", lock) {
		t.Error("expected not hand-edited right after save")
	}

	os.WriteFile(ScriptPath(tmp, "build"), []byte("#!/bin/bash\ngo build -v ."), 0o644)
	if !IsHandEdited(tmp, "build", lock) {
		t.Error("expected hand-edited after modifying script")
	}
}

func TestMapsEqual(t *testing.T) {
	a := map[string]string{"x": "1", "y": "2"}
	b := map[string]string{"x": "1", "y": "2"}
	if !mapsEqual(a, b) {
		t.Error("expected equal")
	}

	c := map[string]string{"x": "1", "y": "3"}
	if mapsEqual(a, c) {
		t.Error("expected not equal (different value)")
	}

	d := map[string]string{"x": "1"}
	if mapsEqual(a, d) {
		t.Error("expected not equal (different length)")
	}
}
