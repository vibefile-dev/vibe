package compiled

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/vibefile-dev/vibe/parser"
)

// LockFile records the inputs that produced a compiled script so we can detect
// when recompilation is needed.
type LockFile struct {
	Recipe       string            `yaml:"recipe"`
	Model        string            `yaml:"model"`
	Variables    map[string]string `yaml:"variables,omitempty"`
	ContextFiles map[string]string `yaml:"context_files,omitempty"` // filename → sha256
	SkillHash    string            `yaml:"skill_hash,omitempty"`
	ScriptHash   string            `yaml:"script_hash"`
	GeneratedAt  time.Time         `yaml:"generated_at"`
}

// CacheDir returns the path to .vibe/compiled/ under the given repo root,
// creating it if it doesn't exist.
func CacheDir(repoRoot string) (string, error) {
	dir := filepath.Join(repoRoot, ".vibe", "compiled")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	return dir, nil
}

// ScriptPath returns the path to the compiled .sh file for a target.
func ScriptPath(repoRoot, targetName string) string {
	return filepath.Join(repoRoot, ".vibe", "compiled", targetName+".sh")
}

// LockPath returns the path to the .lock file for a target.
func LockPath(repoRoot, targetName string) string {
	return filepath.Join(repoRoot, ".vibe", "compiled", targetName+".lock")
}

// BuildLock creates a LockFile from the current inputs for a target.
// skillInstructions is the resolved SKILL.md content (empty if no skill is used).
func BuildLock(target *parser.Target, model string, vars map[string]string, contextFiles map[string]string, skillInstructions, script string) *LockFile {
	lock := &LockFile{
		Recipe:       parser.SubstituteVars(target.Recipe, vars),
		Model:        model,
		Variables:    make(map[string]string),
		ContextFiles: make(map[string]string),
		ScriptHash:   hash([]byte(script)),
		GeneratedAt:  time.Now().UTC(),
	}

	if skillInstructions != "" {
		lock.SkillHash = hash([]byte(skillInstructions))
	}

	for k, v := range vars {
		lock.Variables[k] = v
	}

	for name, content := range contextFiles {
		lock.ContextFiles[name] = hash([]byte(content))
	}

	return lock
}

// Save writes the compiled script and its lock file to disk.
func Save(repoRoot, targetName, script string, lock *LockFile) error {
	dir, err := CacheDir(repoRoot)
	if err != nil {
		return err
	}

	scriptPath := filepath.Join(dir, targetName+".sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		return fmt.Errorf("write compiled script: %w", err)
	}

	lockData, err := yaml.Marshal(lock)
	if err != nil {
		return fmt.Errorf("marshal lock file: %w", err)
	}
	lockPath := filepath.Join(dir, targetName+".lock")
	if err := os.WriteFile(lockPath, lockData, 0o644); err != nil {
		return fmt.Errorf("write lock file: %w", err)
	}

	return nil
}

// Load reads the compiled script for a target. Returns the script contents and
// true if a cached script exists, or empty string and false if not.
func Load(repoRoot, targetName string) (string, bool) {
	data, err := os.ReadFile(ScriptPath(repoRoot, targetName))
	if err != nil {
		return "", false
	}
	return string(data), true
}

// LoadLock reads the lock file for a target.
func LoadLock(repoRoot, targetName string) (*LockFile, error) {
	data, err := os.ReadFile(LockPath(repoRoot, targetName))
	if err != nil {
		return nil, err
	}
	var lock LockFile
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse lock file: %w", err)
	}
	return &lock, nil
}

// IsValid checks whether the cached compiled output is still valid given
// current inputs. Returns true if the cache can be used, false if recompilation
// is needed. The reason string explains what changed.
// skillInstructions is the current resolved SKILL.md content (empty if no skill).
func IsValid(lock *LockFile, target *parser.Target, model string, vars map[string]string, contextFiles map[string]string, skillInstructions string) (bool, string) {
	currentRecipe := parser.SubstituteVars(target.Recipe, vars)
	if lock.Recipe != currentRecipe {
		return false, "recipe changed"
	}

	if lock.Model != model {
		return false, fmt.Sprintf("model changed (%s → %s)", lock.Model, model)
	}

	if !mapsEqual(lock.Variables, vars) {
		return false, "variables changed"
	}

	currentSkillHash := ""
	if skillInstructions != "" {
		currentSkillHash = hash([]byte(skillInstructions))
	}
	if lock.SkillHash != currentSkillHash {
		return false, "skill changed"
	}

	for name, content := range contextFiles {
		currentHash := hash([]byte(content))
		if locked, ok := lock.ContextFiles[name]; !ok {
			return false, fmt.Sprintf("new context file: %s", name)
		} else if locked != currentHash {
			return false, fmt.Sprintf("context file changed: %s", name)
		}
	}
	for name := range lock.ContextFiles {
		if _, ok := contextFiles[name]; !ok {
			return false, fmt.Sprintf("context file removed: %s", name)
		}
	}

	return true, ""
}

// IsHandEdited checks whether the compiled script on disk differs from the
// script hash recorded in the lock file. This detects manual edits.
func IsHandEdited(repoRoot, targetName string, lock *LockFile) bool {
	data, err := os.ReadFile(ScriptPath(repoRoot, targetName))
	if err != nil {
		return false
	}
	return hash(data) != lock.ScriptHash
}

// Status describes the cache state of a single target.
type Status struct {
	Name        string
	Compiled    bool
	Valid       bool
	HandEdited  bool
	Reason      string // why invalid, if applicable
	GeneratedAt time.Time
}

// GetStatus returns the cache status for a single target.
func GetStatus(repoRoot string, target *parser.Target, model string, vars map[string]string, contextFiles map[string]string, skillInstructions string) Status {
	s := Status{Name: target.Name}

	lock, err := LoadLock(repoRoot, target.Name)
	if err != nil {
		return s
	}

	if _, exists := Load(repoRoot, target.Name); !exists {
		return s
	}

	s.Compiled = true
	s.GeneratedAt = lock.GeneratedAt
	s.HandEdited = IsHandEdited(repoRoot, target.Name, lock)

	valid, reason := IsValid(lock, target, model, vars, contextFiles, skillInstructions)
	s.Valid = valid
	s.Reason = reason

	return s
}

func hash(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", h)
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	keysA := sortedKeys(a)
	keysB := sortedKeys(b)
	for i := range keysA {
		if keysA[i] != keysB[i] || a[keysA[i]] != b[keysB[i]] {
			return false
		}
	}
	return true
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
