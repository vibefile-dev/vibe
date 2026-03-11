package skill

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillInfo holds parsed metadata and content from a SKILL.md file.
type SkillInfo struct {
	Name        string // skill name from @skill directive
	Description string // from YAML frontmatter (empty if no frontmatter)
	Content     string // markdown body (without frontmatter)
	RawContent  string // full file content (frontmatter + body, for uploading)
	FilePath    string // absolute path to the SKILL.md file
	Hash        string // sha256 of full file for cache invalidation
}

type frontmatter struct {
	Description string `yaml:"description"`
}

// Resolve locates a skill by name and returns parsed skill info.
// Resolution order:
//  1. skills/<name>/SKILL.md in the repo root
//  2. .vibe/skills/<name>/SKILL.md in the repo root
//  3. ~/.vibe/skills/<name>/SKILL.md (user-global)
//  4. Additional paths from extraSources (typically from .vibe/config.yaml)
func Resolve(repoRoot, skillName string, extraSources []string) (*SkillInfo, error) {
	candidates := []string{
		filepath.Join(repoRoot, "skills", skillName, "SKILL.md"),
		filepath.Join(repoRoot, ".vibe", "skills", skillName, "SKILL.md"),
	}

	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".vibe", "skills", skillName, "SKILL.md"))
	}

	for _, src := range extraSources {
		expanded := expandPath(src)
		candidates = append(candidates, filepath.Join(expanded, skillName, "SKILL.md"))
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		raw := strings.TrimSpace(string(data))
		if raw == "" {
			continue
		}

		desc, body := parseFrontmatter(raw)
		h := sha256.Sum256(data)

		return &SkillInfo{
			Name:        skillName,
			Description: desc,
			Content:     body,
			RawContent:  raw,
			FilePath:    path,
			Hash:        fmt.Sprintf("sha256:%x", h),
		}, nil
	}

	return nil, fmt.Errorf("skill %q not found (searched: %s)", skillName, strings.Join(candidates, ", "))
}

// parseFrontmatter splits a SKILL.md file into YAML frontmatter description
// and markdown body. Returns empty description if no frontmatter is present.
func parseFrontmatter(raw string) (description, body string) {
	if !strings.HasPrefix(raw, "---\n") && !strings.HasPrefix(raw, "---\r\n") {
		return "", raw
	}

	rest := raw[4:]
	endIdx := strings.Index(rest, "\n---")
	if endIdx < 0 {
		return "", raw
	}

	fmContent := rest[:endIdx]
	body = strings.TrimSpace(rest[endIdx+4:])

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(fmContent), &fm); err != nil {
		return "", raw
	}

	return fm.Description, body
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
