package cli

import (
	"fmt"
	"os"

	"github.com/lignumqt/envsnap/internal/export"
	"github.com/lignumqt/envsnap/internal/storage"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	var (
		format       string
		outputPath   string
		skipPackages bool
		skipEnv      bool
		skipModules  bool
		onlyLoaded   bool
	)

	cmd := &cobra.Command{
		Use:   "export <snapshot>",
		Short: "Export a snapshot as a reproducible environment artifact",
		Long: `Converts a snapshot into a ready-to-use artifact for reproducing the
captured environment on another machine.

Available formats:
  script      Bash script: installs packages & Go, loads kernel modules, exports env vars
  dockerfile  Dockerfile: FROM base + RUN install + ENV (modules listed as comments)
  ansible     Ansible playbook YAML: apt/dnf task + modprobe task + env persistence
                Requires: ansible-galaxy collection install community.general
  env         Shell env fragment: "export KEY=VALUE" lines, sourceable directly

By default output goes to stdout — pipe it or use --output to write to a file.

Examples:
  envsnap export snap.json                            bash setup script → stdout
  envsnap export snap.json -o setup.sh                write script to file
  envsnap export snap.json -f dockerfile -o Dockerfile && docker build -t my-env .
  envsnap export snap.json -f ansible -o playbook.yml
  source <(envsnap export snap.json -f env)           inject env into current shell
  envsnap export snap.json --skip-packages -f script  script without package section`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			snap, err := storage.Load(args[0])
			if err != nil {
				return err
			}

			opts := export.Options{
				Format:       export.Format(format),
				SkipPackages: skipPackages,
				SkipEnv:      skipEnv,
				SkipModules:  skipModules,
				OnlyLoaded:   onlyLoaded,
			}

			result, err := export.Export(snap, opts)
			if err != nil {
				return err
			}

			if outputPath == "" {
				fmt.Fprint(os.Stdout, result)
				return nil
			}

			if err := os.WriteFile(outputPath, []byte(result), 0o644); err != nil {
				return fmt.Errorf("writing output: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Written to %s\n", outputPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "script", "output format: script, dockerfile, ansible, env")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "write to file instead of stdout")
	cmd.Flags().BoolVar(&skipPackages, "skip-packages", false, "omit package-installation section")
	cmd.Flags().BoolVar(&skipEnv, "skip-env", false, "omit environment-variables section")
	cmd.Flags().BoolVar(&skipModules, "skip-modules", false, "omit kernel-modules section")
	cmd.Flags().BoolVar(&onlyLoaded, "only-loaded", false, "include only currently loaded kernel modules (not installed-only)")

	return cmd
}
