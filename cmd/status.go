package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/vibefile-dev/vibe/compiled"
	"github.com/vibefile-dev/vibe/config"
	vibecontext "github.com/vibefile-dev/vibe/context"
	"github.com/vibefile-dev/vibe/skill"
	"github.com/vibefile-dev/vibe/ui"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show compiled/uncompiled state of all targets",
	Args:  cobra.NoArgs,
	RunE:  showStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

var (
	statusGreen  = color.New(color.FgGreen)
	statusYellow = color.New(color.FgYellow)
	statusDim    = color.New(color.Faint)
	statusCyan   = color.New(color.FgCyan)
	statusBold   = color.New(color.Bold)
)

func showStatus(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	vf, err := loadVibefile(repoRoot)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	ui.Header("  Target status")
	fmt.Fprintln(os.Stderr)

	maxName := 0
	for _, name := range vf.Order {
		if len(name) > maxName {
			maxName = len(name)
		}
	}

	projCfg, err := config.LoadProjectConfig(repoRoot)
	if err != nil {
		projCfg = &config.ProjectConfig{}
	}

	for _, name := range vf.Order {
		target := vf.Targets[name]
		model := config.ResolveModel(target.Model, vf.Variables["model"])

		if target.ExecutionMode() == "agent" {
			ui.StatusLine(name, statusDim.Sprint("⊘"), statusDim.Sprint("agent (not compiled)"), maxName)
			continue
		}

		ctx, err := vibecontext.Collect(repoRoot, target)
		if err != nil {
			ui.StatusLine(name, color.RedString("?"), fmt.Sprintf("context error: %v", err), maxName)
			continue
		}

		skillRawContent := ""
		if target.HasDirective("skill") {
			skillName := target.DirectiveArgs("skill")
			info, err := skill.Resolve(repoRoot, skillName, projCfg.SkillSources)
			if err == nil {
				skillRawContent = info.RawContent
			}
		}

		s := compiled.GetStatus(repoRoot, target, model, vf.Variables, ctx.HashableFiles(), skillRawContent)

		if !s.Compiled {
			ui.StatusLine(name, statusDim.Sprint("○"), statusDim.Sprint("not compiled"), maxName)
			continue
		}

		if !s.Valid {
			ui.StatusLine(name, statusYellow.Sprint("↻"), statusYellow.Sprintf("stale (%s)", s.Reason), maxName)
			continue
		}

		extra := ""
		if s.HandEdited {
			extra = statusCyan.Sprint(" [hand-edited]")
		}
		ui.StatusLine(name, statusGreen.Sprint("●"), fmt.Sprintf("compiled %s%s", s.GeneratedAt.Format("2006-01-02 15:04"), extra), maxName)
	}

	fmt.Fprintln(os.Stderr)
	return nil
}
