package cli

import (
	"sort"

	"github.com/lignumqt/envsnap/internal/diff"
	"github.com/lignumqt/envsnap/internal/storage"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "diff <snapshotA> <snapshotB>",
		Short: "Compare two snapshots and show differences",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			snapA, err := storage.Load(args[0])
			if err != nil {
				return err
			}
			snapB, err := storage.Load(args[1])
			if err != nil {
				return err
			}

			result := diff.Diff(snapA, snapB)
			printDiff(result, limit)
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "max number of entries to show per category (0 = unlimited)")
	return cmd
}

func printDiff(result diff.DiffResult, limit int) {
	if result.IsClean() {
		pterm.Success.Println("Snapshots are identical — no differences found.")
		return
	}

	pterm.DefaultHeader.WithFullWidth().Println("Snapshot Diff")

	printSection := func(title string, items []string, color func(...interface{}) string) {
		if len(items) == 0 {
			return
		}
		pterm.DefaultSection.Println(title)
		sort.Strings(items)
		shown := items
		if limit > 0 && len(items) > limit {
			shown = items[:limit]
		}
		for _, line := range shown {
			pterm.Println(color(line))
		}
		if limit > 0 && len(items) > limit {
			pterm.Info.Printf("… and %d more (use --limit 0 to show all)\n", len(items)-limit)
		}
	}

	printSection("Warnings", result.Warnings, pterm.Yellow)
	printSection("Added", result.Added, pterm.Green)
	printSection("Removed", result.Removed, pterm.Red)
	printSection("Changed", result.Changed, pterm.Cyan)

	pterm.Println()
	pterm.DefaultTable.WithHasHeader().WithData(pterm.TableData{
		{"Category", "Count"},
		{"Warnings", pterm.Yellow(len(result.Warnings))},
		{"Added", pterm.Green(len(result.Added))},
		{"Removed", pterm.Red(len(result.Removed))},
		{"Changed", pterm.Cyan(len(result.Changed))},
	}).Render() //nolint:errcheck
}
