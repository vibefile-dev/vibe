package vercel

import (
	"log/slog"
	"path/filepath"

	"github.com/vibefile-dev/vibe/detect"
)

func init() { detect.RegisterAddon(&Addon{}) }

// Addon detects Vercel configuration and contributes deploy targets.
type Addon struct{}

func (a *Addon) Name() string { return "vercel" }

func (a *Addon) Detect(repoRoot string) *detect.AddonResult {
	vercelJSON := filepath.Join(repoRoot, "vercel.json")
	if !detect.FileExists(vercelJSON) {
		return nil
	}
	slog.Debug("vercel addon: vercel.json found", "path", vercelJSON)

	return &detect.AddonResult{
		Label: "Vercel",
		Targets: []detect.TemplateTarget{
			{
				Name:    "deploy",
				Section: "infrastructure",
				Recipe:  "deploy to Vercel using the Vercel CLI, creating a production deployment",
			},
		},
	}
}
