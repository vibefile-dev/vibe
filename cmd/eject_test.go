package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEjectBasicMakefile(t *testing.T) {
	dir := t.TempDir()

	// Write a Vibefile
	vibefile := `model = claude-sonnet-4-6
env = production

build:
    "compile the project for $(env)"

test:
    "run all tests"

deploy: test build:
    "deploy to $(env)"
    @mcp fly-mcp
`
	if err := os.WriteFile(filepath.Join(dir, "Vibefile"), []byte(vibefile), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write compiled scripts
	compiledDir := filepath.Join(dir, ".vibe", "compiled")
	if err := os.MkdirAll(compiledDir, 0o755); err != nil {
		t.Fatal(err)
	}

	buildScript := `#!/bin/bash
set -euo pipefail

echo "building for $(env)"
go build ./...
`
	testScript := `#!/bin/bash
set -euo pipefail

go test -v ./...
`
	if err := os.WriteFile(filepath.Join(compiledDir, "build.sh"), []byte(buildScript), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(compiledDir, "test.sh"), []byte(testScript), 0o644); err != nil {
		t.Fatal(err)
	}

	// Change to temp dir and run eject
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := ejectMakefile(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("ejectMakefile returned error: %v", err)
	}

	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Check the output contains expected elements
	if !strings.Contains(output, "SHELL := /bin/bash") {
		t.Error("expected SHELL declaration")
	}
	if !strings.Contains(output, ".SHELLFLAGS := -euo pipefail -c") {
		t.Error("expected .SHELLFLAGS declaration")
	}
	if !strings.Contains(output, ".ONESHELL:") {
		t.Error("expected .ONESHELL directive")
	}
	if !strings.Contains(output, "ENV := production") {
		t.Error("expected ENV variable")
	}
	if !strings.Contains(output, "build:") {
		t.Error("expected build target")
	}
	if !strings.Contains(output, "test:") {
		t.Error("expected test target")
	}
	if !strings.Contains(output, "deploy: skipped (agent mode") {
		t.Error("expected deploy to be skipped as agent target")
	}
	// Should not contain shebang
	if strings.Contains(output, "#!/bin/bash") {
		t.Error("should not contain shebang lines")
	}
	// Should not contain set -euo pipefail (handled by .SHELLFLAGS)
	if strings.Contains(output, "set -euo pipefail") {
		t.Error("should not contain set -euo pipefail (handled by .SHELLFLAGS)")
	}
	// Should contain .PHONY
	if !strings.Contains(output, ".PHONY:") {
		t.Error("expected .PHONY declaration")
	}
	// Should have help target
	if !strings.Contains(output, "help:") {
		t.Error("expected help target")
	}
}

func TestEjectPreflightSeparation(t *testing.T) {
	dir := t.TempDir()

	vibefile := `model = claude-sonnet-4-6

build:
    "compile the project"
`
	if err := os.WriteFile(filepath.Join(dir, "Vibefile"), []byte(vibefile), 0o644); err != nil {
		t.Fatal(err)
	}

	compiledDir := filepath.Join(dir, ".vibe", "compiled")
	if err := os.MkdirAll(compiledDir, 0o755); err != nil {
		t.Fatal(err)
	}

	script := `#!/bin/bash
set -euo pipefail

# Preflight checks
if ! command -v go &> /dev/null; then
  echo "error: go is required but not installed"
  exit 2
fi

# Build the binary
go build -o myapp ./cmd/app
`
	if err := os.WriteFile(filepath.Join(compiledDir, "build.sh"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := ejectMakefile(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("ejectMakefile returned error: %v", err)
	}

	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Should have a _preflight-build target
	if !strings.Contains(output, "_preflight-build:") {
		t.Errorf("expected _preflight-build target, got:\n%s", output)
	}
	// The _preflight-build target should contain the command -v check
	if !strings.Contains(output, "command -v go") {
		t.Errorf("expected command -v go in preflight target, got:\n%s", output)
	}
	// The main build target should depend on _preflight-build
	if !strings.Contains(output, "build: _preflight-build") {
		t.Errorf("expected build to depend on _preflight-build, got:\n%s", output)
	}
	// The build target body should NOT contain the preflight check
	// (it should be in the _preflight-build target instead)
	// Find the build: line and check its recipe doesn't have "command -v"
	lines := strings.Split(output, "\n")
	inBuild := false
	for _, line := range lines {
		if strings.HasPrefix(line, "build:") {
			inBuild = true
			continue
		}
		if inBuild {
			if !strings.HasPrefix(line, "\t") && line != "" {
				break
			}
			if strings.Contains(line, "command -v") {
				t.Errorf("build target body should not contain preflight checks, found: %s", line)
			}
		}
	}
	// Should have section headers
	if !strings.Contains(output, "# ── preflight checks") {
		t.Error("expected preflight section header")
	}
	if !strings.Contains(output, "# ── targets") {
		t.Error("expected targets section header")
	}
}

func TestEjectNoPreflightWhenAbsent(t *testing.T) {
	dir := t.TempDir()

	vibefile := `model = claude-sonnet-4-6

clean:
    "remove build artifacts"
`
	if err := os.WriteFile(filepath.Join(dir, "Vibefile"), []byte(vibefile), 0o644); err != nil {
		t.Fatal(err)
	}

	compiledDir := filepath.Join(dir, ".vibe", "compiled")
	if err := os.MkdirAll(compiledDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Script with no preflight section
	script := `#!/bin/bash
set -euo pipefail

rm -rf dist/
echo "Cleaned."
`
	if err := os.WriteFile(filepath.Join(compiledDir, "clean.sh"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := ejectMakefile(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("ejectMakefile returned error: %v", err)
	}

	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Should NOT have any preflight targets or section headers
	if strings.Contains(output, "_preflight-") {
		t.Error("should not have preflight targets when scripts have no preflight section")
	}
	if strings.Contains(output, "# ── preflight") {
		t.Error("should not have preflight section header when no preflight exists")
	}
	// Should still have the clean target
	if !strings.Contains(output, "clean:") {
		t.Error("expected clean target")
	}
}

func TestSplitPreflight(t *testing.T) {
	script := `#!/bin/bash
set -euo pipefail

# Preflight checks
if ! command -v go &> /dev/null; then
  echo "error: go is required but not installed"
  exit 2
fi

# Build the binary
go build -o myapp .
echo "Done"
`
	preflight, body := splitPreflight(script)

	if len(preflight) == 0 {
		t.Fatal("expected preflight lines")
	}

	// Preflight should contain the command check
	preflightStr := strings.Join(preflight, "\n")
	if !strings.Contains(preflightStr, "command -v go") {
		t.Errorf("preflight should contain 'command -v go', got:\n%s", preflightStr)
	}

	// Body should contain the build command
	bodyStr := strings.Join(body, "\n")
	if !strings.Contains(bodyStr, "go build") {
		t.Errorf("body should contain 'go build', got:\n%s", bodyStr)
	}

	// Body should NOT contain the command check
	if strings.Contains(bodyStr, "command -v go") {
		t.Errorf("body should NOT contain command check, got:\n%s", bodyStr)
	}

	// Neither should contain shebang or set -euo
	if strings.Contains(preflightStr, "#!/bin/bash") || strings.Contains(bodyStr, "#!/bin/bash") {
		t.Error("should strip shebang")
	}
	if strings.Contains(preflightStr, "set -euo pipefail") || strings.Contains(bodyStr, "set -euo pipefail") {
		t.Error("should strip set -euo pipefail")
	}
}

func TestSplitPreflightNoSection(t *testing.T) {
	script := `#!/bin/bash
set -euo pipefail

rm -rf dist/
echo "Cleaned"
`
	preflight, body := splitPreflight(script)

	if len(preflight) != 0 {
		t.Errorf("expected no preflight lines, got %d: %v", len(preflight), preflight)
	}

	bodyStr := strings.Join(body, "\n")
	if !strings.Contains(bodyStr, "rm -rf dist/") {
		t.Errorf("body should contain 'rm -rf dist/', got:\n%s", bodyStr)
	}
}

func TestEjectNoCompiledScripts(t *testing.T) {
	dir := t.TempDir()

	vibefile := `model = claude-sonnet-4-6

build:
    "compile the project"
`
	if err := os.WriteFile(filepath.Join(dir, "Vibefile"), []byte(vibefile), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := ejectMakefile(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("ejectMakefile returned error: %v", err)
	}

	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "skipped (no compiled script found)") {
		t.Error("expected skip comment for uncompiled target")
	}
}

func TestEjectOutputFlag(t *testing.T) {
	dir := t.TempDir()

	vibefile := `model = claude-sonnet-4-6

build:
    "compile the project"
`
	if err := os.WriteFile(filepath.Join(dir, "Vibefile"), []byte(vibefile), 0o644); err != nil {
		t.Fatal(err)
	}

	compiledDir := filepath.Join(dir, ".vibe", "compiled")
	if err := os.MkdirAll(compiledDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(compiledDir, "build.sh"), []byte("#!/bin/bash\nset -euo pipefail\ngo build ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	outFile := filepath.Join(dir, "Makefile")
	ejectOutput = outFile
	defer func() { ejectOutput = "" }()

	err := ejectMakefile(nil, nil)
	if err != nil {
		t.Fatalf("ejectMakefile returned error: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("could not read output Makefile: %v", err)
	}

	output := string(data)
	if !strings.Contains(output, "build:") {
		t.Error("expected build target in output file")
	}
	if !strings.Contains(output, "go build ./...") {
		t.Error("expected go build command in output file")
	}
}

func TestConvertVarRefs(t *testing.T) {
	vars := map[string]string{
		"env":     "production",
		"project": "myapp",
		"model":   "claude-sonnet-4-6",
	}

	line := `echo "deploying $(env) for $(project)"`
	result := convertVarRefs(line, vars)

	if !strings.Contains(result, "$(ENV)") {
		t.Errorf("expected $(ENV) in result, got: %s", result)
	}
	if !strings.Contains(result, "$(PROJECT)") {
		t.Errorf("expected $(PROJECT) in result, got: %s", result)
	}
	// model should not be converted
	if strings.Contains(result, "$(MODEL)") {
		t.Errorf("model variable should not be converted, got: %s", result)
	}
}

func TestSortedVarKeys(t *testing.T) {
	vars := map[string]string{
		"zoo":   "z",
		"alpha": "a",
		"beta":  "b",
	}
	keys := sortedVarKeys(vars)
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0] != "alpha" || keys[1] != "beta" || keys[2] != "zoo" {
		t.Errorf("expected [alpha beta zoo], got %v", keys)
	}
}
