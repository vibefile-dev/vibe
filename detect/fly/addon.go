package fly

import (
	"log/slog"
	"path/filepath"

	"github.com/vibefile-dev/vibe/detect"
)

func init() { detect.RegisterAddon(&Addon{}) }

// Addon detects Fly.io configuration and contributes deploy targets.
type Addon struct{}

func (a *Addon) Name() string { return "fly" }

func (a *Addon) Detect(repoRoot string) *detect.AddonResult {
	flyToml := filepath.Join(repoRoot, "fly.toml")
	if !detect.FileExists(flyToml) {
		return nil
	}
	slog.Debug("fly addon: fly.toml found", "path", flyToml)

	return &detect.AddonResult{
		Label: "Fly.io",
		Targets: []detect.TemplateTarget{
			{
				Name:       "deploy",
				Section:    "infrastructure",
				Recipe:     "deploy to Fly.io using flyctl and verify the app came up healthy",
				Directives: []string{"@require clean git status"},
			},
		},
	}
}
