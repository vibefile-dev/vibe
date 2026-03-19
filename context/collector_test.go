package context

import (
	"strings"
	"testing"

	"github.com/vibefile-dev/vibe/parser"
)

func TestHashableFilesEmpty(t *testing.T) {
	c := &Collected{ProjectFiles: make(map[string]string)}
	files := c.HashableFiles()
	if len(files) != 0 {
		t.Errorf("expected 0 hashable files, got %d", len(files))
	}
}

func TestHashableFilesIncludesVibefile(t *testing.T) {
	c := &Collected{
		VibefileRaw:  "model = claude\n",
		ProjectFiles: make(map[string]string),
	}
	files := c.HashableFiles()
	if files["Vibefile"] != "model = claude\n" {
		t.Error("expected Vibefile in hashable files")
	}
}

func TestHashableFilesMergesProjectFiles(t *testing.T) {
	c := &Collected{
		VibefileRaw:  "model = claude\n",
		ProjectFiles: map[string]string{"go.mod": "module example"},
	}
	files := c.HashableFiles()
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
	if files["go.mod"] != "module example" {
		t.Error("expected go.mod in hashable files")
	}
}

func TestFormatContainsFileTree(t *testing.T) {
	c := &Collected{
		FileTree:     "main.go\ncmd/\n",
		ProjectFiles: make(map[string]string),
	}
	out := c.Format()
	if !strings.Contains(out, "## File tree") {
		t.Error("expected file tree header")
	}
	if !strings.Contains(out, "main.go") {
		t.Error("expected file tree content")
	}
}

func TestFormatContainsGitStatus(t *testing.T) {
	c := &Collected{
		FileTree:     ".\n",
		GitStatus:    "## main",
		ProjectFiles: make(map[string]string),
	}
	out := c.Format()
	if !strings.Contains(out, "## Git status") {
		t.Error("expected git status header")
	}
}

func TestFormatOmitsGitStatusWhenEmpty(t *testing.T) {
	c := &Collected{
		FileTree:     ".\n",
		ProjectFiles: make(map[string]string),
	}
	out := c.Format()
	if strings.Contains(out, "Git status") {
		t.Error("expected no git status when empty")
	}
}

func TestFormatTruncatesLargeFiles(t *testing.T) {
	large := strings.Repeat("x", 5000)
	c := &Collected{
		FileTree:     ".\n",
		ProjectFiles: map[string]string{"big.txt": large},
	}
	out := c.Format()
	if !strings.Contains(out, "truncated") {
		t.Error("expected truncation notice for large file")
	}
}

func TestContainsAny(t *testing.T) {
	if !containsAny("deploy to production", "deploy", "ship") {
		t.Error("expected match on 'deploy'")
	}
	if containsAny("build the app", "deploy", "ship") {
		t.Error("expected no match")
	}
	if !containsAny("shipping now", "deploy", "ship") {
		t.Error("expected match on 'ship'")
	}
}

func TestInferTaskFilesDeployTarget(t *testing.T) {
	target := &parser.Target{Name: "deploy", Recipe: "deploy to fly.io"}
	files := inferTaskFiles(target)
	found := false
	for _, f := range files {
		if f == "fly.toml" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected fly.toml for deploy target")
	}
}

func TestInferTaskFilesTestTarget(t *testing.T) {
	target := &parser.Target{Name: "test", Recipe: "run the tests"}
	files := inferTaskFiles(target)
	found := false
	for _, f := range files {
		if f == "jest.config.js" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected jest.config.js for test target")
	}
}

func TestInferTaskFilesBuildTarget(t *testing.T) {
	target := &parser.Target{Name: "build", Recipe: "compile the app"}
	files := inferTaskFiles(target)
	found := false
	for _, f := range files {
		if f == "tsconfig.json" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected tsconfig.json for build target")
	}
}

func TestInferTaskFilesNoMatchReturnsEmpty(t *testing.T) {
	target := &parser.Target{Name: "hello", Recipe: "say hi"}
	files := inferTaskFiles(target)
	if len(files) != 0 {
		t.Errorf("expected no inferred files, got %v", files)
	}
}
func TestInferTaskFilesIncludesShellScripts(t *testing.T) {
	target := &parser.Target{Name: "build", Recipe: "compile the app"}
	files := inferTaskFiles(target)
	found := false
	for _, f := range files {
		if f == "build.sh" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected build.sh to be included for build target")
	}
}

func TestInferTaskFilesDeployIncludesDeploySh(t *testing.T) {
	target := &parser.Target{Name: "deploy", Recipe: "deploy to fly.io"}
	files := inferTaskFiles(target)
	found := false
	for _, f := range files {
		if f == "deploy.sh" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected deploy.sh to be included for deploy target")
	}
}
