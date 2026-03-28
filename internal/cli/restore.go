package cli

import (
	"fmt"

	"github.com/lignumqt/envsnap/internal/restore"
	"github.com/lignumqt/envsnap/internal/storage"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func newRestoreCmd() *cobra.Command {
	var (
		dryRun bool
		yes    bool
	)

	cmd := &cobra.Command{
		Use:   "restore <snapshot>",
		Short: "Restore environment from a snapshot",
		Long: `Builds a restore plan from the snapshot and optionally applies it.

By default runs in dry-run mode showing what would be done.
Use --yes to actually execute the restore (installs missing packages with sudo).

Kernel modules are always shown as informational only — apply them manually with modprobe.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			snap, err := storage.Load(args[0])
			if err != nil {
				return err
			}

			plan, err := restore.Build(snap)
			if err != nil {
				return fmt.Errorf("build restore plan: %w", err)
			}

			if len(plan.Actions) == 0 {
				pterm.Success.Println("Nothing to restore — environment is already up to date.")
				return nil
			}

			pterm.DefaultSection.Printf("Restore Plan (%d actions)\n", len(plan.Actions))
			printPlan(plan)

			// If neither dry-run nor --yes, default to dry-run.
			if !yes {
				dryRun = true
				pterm.Info.Println("Dry-run mode. Use --yes to apply.")
			}

			if dryRun {
				return nil
			}

			pterm.Warning.Println("Applying restore plan (this may require sudo)…")
			return restore.Apply(plan, false)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be done without making changes")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "apply the restore plan (install missing packages)")

	return cmd
}

func printPlan(plan *restore.Plan) {
	table := pterm.TableData{{"#", "Type", "Action"}}
	for i, action := range plan.Actions {
		table = append(table, []string{
			fmt.Sprintf("%d", i+1),
			string(action.Type),
			action.Description,
		})
	}
	_ = pterm.DefaultTable.WithHasHeader().WithData(table).Render()
}
