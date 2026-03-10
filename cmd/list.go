package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/vibefiledev/vibe/parser"
	"github.com/vibefiledev/vibe/ui"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all targets with their descriptions",
	Args:  cobra.NoArgs,
	RunE:  listTargets,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

var (
	listBold = color.New(color.Bold)
	listDim  = color.New(color.Faint)
	listCyan = color.New(color.FgCyan)
)

func listTargets(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	vf, err := loadVibefile(repoRoot)
	if err != nil {
		return err
	}

	maxName := 0
	for _, name := range vf.Order {
		if len(name) > maxName {
			maxName = len(name)
		}
	}

	fmt.Fprintln(os.Stderr)
	ui.Header("  Targets")
	fmt.Fprintln(os.Stderr)

	for _, name := range vf.Order {
		t := vf.Targets[name]
		recipe := parser.SubstituteVars(t.Recipe, vf.Variables)
		if recipe == "" {
			recipe = fmt.Sprintf("(@skill %s)", t.DirectiveArgs("skill"))
		}

		padding := strings.Repeat(" ", maxName-len(name)+2)
		fmt.Fprintf(os.Stderr, "  %s%s%s\n", listBold.Sprint(name), padding, recipe)

		if len(t.Dependencies) > 0 {
			depPadding := strings.Repeat(" ", maxName+4)
			fmt.Fprintf(os.Stderr, "  %s%s\n", depPadding, listDim.Sprintf("depends on: %s", strings.Join(t.Dependencies, ", ")))
		}

		mode := t.ExecutionMode()
		if mode != "codegen" {
			modePadding := strings.Repeat(" ", maxName+4)
			fmt.Fprintf(os.Stderr, "  %s%s\n", modePadding, listCyan.Sprintf("mode: %s", mode))
		}
	}

	fmt.Fprintln(os.Stderr)
	return nil
}
