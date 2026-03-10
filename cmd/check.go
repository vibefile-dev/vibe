package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/vibefiledev/vibe/resolver"
	"github.com/vibefiledev/vibe/ui"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate the Vibefile without running anything",
	Args:  cobra.NoArgs,
	RunE:  checkVibefile,
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

func checkVibefile(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	sp := ui.NewSpinner("Validating Vibefile…")

	vf, err := loadVibefile(repoRoot)
	if err != nil {
		sp.Fail(fmt.Sprintf("Parse error: %v", err))
		return err
	}

	for _, name := range vf.Order {
		if _, err := resolver.Resolve(vf, name); err != nil {
			sp.Fail(fmt.Sprintf("Dependency error: %v", err))
			return err
		}
	}

	sp.Success(fmt.Sprintf("Vibefile is valid (%d targets)", len(vf.Targets)))

	warnings := 0
	for _, name := range vf.Order {
		t := vf.Targets[name]
		if t.ExecutionMode() == "agent" {
			ui.Warn(fmt.Sprintf("%s: @mcp targets are not yet supported", name))
			warnings++
		}
		if t.ExecutionMode() == "skill" && t.Recipe == "" {
			ui.Warn(fmt.Sprintf("%s: @skill resolution is not yet supported", name))
			warnings++
		}
	}

	if warnings > 0 {
		fmt.Fprintf(os.Stderr, "\n")
		ui.Info(fmt.Sprintf("%d warning(s)", warnings))
	}

	return nil
}
