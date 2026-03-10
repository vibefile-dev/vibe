package docker

import (
	"log/slog"
	"path/filepath"

	"github.com/vibefiledev/vibe/detect"
)

func init() { detect.RegisterAddon(&Addon{}) }

// Addon detects Dockerfiles and contributes docker build targets.
type Addon struct{}

func (a *Addon) Name() string { return "docker" }

func (a *Addon) Detect(repoRoot string) *detect.AddonResult {
	dockerfile := filepath.Join(repoRoot, "Dockerfile")
	if !detect.FileExists(dockerfile) {
		return nil
	}
	slog.Debug("docker addon: Dockerfile found", "path", dockerfile)

	return &detect.AddonResult{
		Label: "Docker",
		Targets: []detect.TemplateTarget{
			{
				Name:    "docker",
				Section: "infrastructure",
				Recipe:  "build the Docker image tagged $(name):latest",
			},
		},
	}
}
