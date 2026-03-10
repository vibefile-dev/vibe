package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vibefile-dev/vibe/parser"
)

func loadVibefile(repoRoot string) (*parser.Vibefile, error) {
	path := filepath.Join(repoRoot, "Vibefile")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no Vibefile found in %s", repoRoot)
		}
		return nil, fmt.Errorf("read Vibefile: %w", err)
	}

	vf, err := parser.Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse Vibefile: %w", err)
	}

	return vf, nil
}
