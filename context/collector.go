package context

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vibefile-dev/vibe/parser"
)

// Collected holds the gathered repo context sent to the LLM.
type Collected struct {
	FileTree     string
	VibefileRaw  string
	ProjectFiles map[string]string // filename → contents
	GitStatus    string
}

// HashableFiles returns the map of all files whose contents should be tracked
// for cache invalidation. This includes the Vibefile and all collected project
// files (but not the file tree or git status, which are ephemeral).
func (c *Collected) HashableFiles() map[string]string {
	files := make(map[string]string, len(c.ProjectFiles)+1)
	if c.VibefileRaw != "" {
		files["Vibefile"] = c.VibefileRaw
	}
	for k, v := range c.ProjectFiles {
		files[k] = v
	}
	return files
}

// Collect gathers repo context relevant to the given target.
func Collect(repoRoot string, target *parser.Target) (*Collected, error) {
	c := &Collected{
		ProjectFiles: make(map[string]string),
	}

	tree, err := fileTree(repoRoot, 2)
	if err != nil {
		return nil, fmt.Errorf("file tree: %w", err)
	}
	c.FileTree = tree

	vfPath := filepath.Join(repoRoot, "Vibefile")
	if data, err := os.ReadFile(vfPath); err == nil {
		c.VibefileRaw = string(data)
	}

	projectIndicators := []string{
		"package.json",
		"pyproject.toml",
		"go.mod",
		"Cargo.toml",
		"requirements.txt",
		"Gemfile",
		"pom.xml",
		"build.gradle",
	}
	for _, f := range projectIndicators {
		path := filepath.Join(repoRoot, f)
		if data, err := os.ReadFile(path); err == nil {
			c.ProjectFiles[f] = string(data)
		}
	}

	taskSpecificFiles := inferTaskFiles(target)
	for _, f := range taskSpecificFiles {
		path := filepath.Join(repoRoot, f)
		if data, err := os.ReadFile(path); err == nil {
			c.ProjectFiles[f] = string(data)
		}
	}

	if gitStatus, err := runGitStatus(repoRoot); err == nil {
		c.GitStatus = gitStatus
	}

	return c, nil
}

func inferTaskFiles(target *parser.Target) []string {
	name := strings.ToLower(target.Name)
	recipe := strings.ToLower(target.Recipe)
	combined := name + " " + recipe

	var files []string

	if containsAny(combined, "deploy", "ship", "release") {
		files = append(files, "fly.toml", "Dockerfile", "docker-compose.yml",
			"railway.json", "vercel.json", "netlify.toml", "Procfile")
	}
	if containsAny(combined, "test") {
		files = append(files, "jest.config.js", "jest.config.ts", "vitest.config.ts",
			"pytest.ini", "setup.cfg", ".pytest.ini")
	}
	if containsAny(combined, "build", "compile", "bundle") {
		files = append(files, "tsconfig.json", "webpack.config.js", "vite.config.ts",
			"Makefile", "CMakeLists.txt")
	}
	if containsAny(combined, "seed", "migrat", "database", "schema") {
		files = append(files, "schema.sql", "schema.prisma", "prisma/schema.prisma")
	}

	return files
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func fileTree(root string, maxDepth int) (string, error) {
	var b strings.Builder
	err := walkTree(&b, root, "", 0, maxDepth)
	return b.String(), err
}

var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, ".venv": true, "__pycache__": true,
	"dist": true, "build": true, ".next": true, "target": true, "vendor": true,
}

func walkTree(b *strings.Builder, path, prefix string, depth, maxDepth int) error {
	if depth > maxDepth {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	dirs := make([]os.DirEntry, 0)
	files := make([]os.DirEntry, 0)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") && depth > 0 {
			continue
		}
		if e.IsDir() {
			if !skipDirs[e.Name()] {
				dirs = append(dirs, e)
			}
		} else {
			files = append(files, e)
		}
	}

	for _, f := range files {
		fmt.Fprintf(b, "%s%s\n", prefix, f.Name())
	}
	for _, d := range dirs {
		fmt.Fprintf(b, "%s%s/\n", prefix, d.Name())
		walkTree(b, filepath.Join(path, d.Name()), prefix+"  ", depth+1, maxDepth)
	}
	return nil
}

func runGitStatus(repoRoot string) (string, error) {
	cmd := exec.Command("git", "status", "--short", "--branch")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Format returns a human-readable summary of the collected context suitable for
// inclusion in an LLM prompt.
func (c *Collected) Format() string {
	var b strings.Builder

	b.WriteString("## File tree\n```\n")
	b.WriteString(c.FileTree)
	b.WriteString("```\n\n")

	if c.GitStatus != "" {
		b.WriteString("## Git status\n```\n")
		b.WriteString(c.GitStatus)
		b.WriteString("\n```\n\n")
	}

	for name, content := range c.ProjectFiles {
		b.WriteString(fmt.Sprintf("## %s\n```\n", name))
		if len(content) > 4000 {
			b.WriteString(content[:4000])
			b.WriteString("\n... (truncated)\n")
		} else {
			b.WriteString(content)
		}
		b.WriteString("\n```\n\n")
	}

	return b.String()
}
