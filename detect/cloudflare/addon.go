package cloudflare

import (
	"log/slog"
	"path/filepath"

	"github.com/vibefiledev/vibe/detect"
)

func init() { detect.RegisterAddon(&Addon{}) }

// Addon detects Cloudflare Workers/Pages configuration and contributes targets.
type Addon struct{}

func (a *Addon) Name() string { return "cloudflare" }

func (a *Addon) Detect(repoRoot string) *detect.AddonResult {
	// wrangler.toml is the standard Cloudflare Workers/Pages config
	wranglerToml := filepath.Join(repoRoot, "wrangler.toml")
	wranglerJSON := filepath.Join(repoRoot, "wrangler.json")

	found := ""
	switch {
	case detect.FileExists(wranglerToml):
		found = wranglerToml
	case detect.FileExists(wranglerJSON):
		found = wranglerJSON
	default:
		return nil
	}
	slog.Debug("cloudflare addon: wrangler config found", "path", found)

	return &detect.AddonResult{
		Label: "Cloudflare",
		Targets: []detect.TemplateTarget{
			{
				Name:    "cf-dev",
				Section: "cloudflare",
				Recipe:  "start the Cloudflare Workers local development server using wrangler dev",
			},
			{
				Name:    "cf-deploy",
				Section: "cloudflare",
				Recipe:  "deploy to Cloudflare Workers using wrangler deploy",
			},
		},
	}
}
