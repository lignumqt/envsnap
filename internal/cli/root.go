package cli

import (
	"fmt"
	"os"

	"github.com/lignumqt/envsnap/internal/version"
	"github.com/spf13/cobra"
)

// DebugMode is set to true when the --debug flag is passed.
// Commands in this package read it to activate verbose diagnostics.
var DebugMode bool

// NewRootCmd builds and returns the root cobra command.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "envsnap",
		Short: "Capture, inspect, diff and restore Linux developer environments",
		Long: `EnvSnapshot — CLI tool for capturing a structured snapshot of your Linux
development environment and comparing or restoring it on another machine.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.Version = version.String()
	cmd.PersistentFlags().BoolVarP(&DebugMode, "debug", "d", false, "enable debug output (show commands, stderr, warnings)")

	cmd.AddCommand(
		newSnapshotCmd(),
		newInspectCmd(),
		newDiffCmd(),
		newRestoreCmd(),
		newExportCmd(),
	)

	return cmd
}

// Execute runs the root command and exits on error.
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
