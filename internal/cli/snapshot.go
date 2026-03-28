package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lignumqt/envsnap/internal/collectors"
	"github.com/lignumqt/envsnap/internal/snapshot"
	"github.com/lignumqt/envsnap/internal/storage"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func newSnapshotCmd() *cobra.Command {
	var (
		outputPath string
		include    string
		exclude    string
		timeout    int
	)

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Capture a snapshot of the current environment",
		Long: `Runs all (or selected) collectors and saves the result to a JSON file.

Available collectors: env, system, go, packages, kernel_modules`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(timeout)*time.Second)
			defer cancel()
			if DebugMode {
				ctx = collectors.WithDebug(ctx)
			}

			opts := snapshot.Options{
				Include: splitCSV(include),
				Exclude: splitCSV(exclude),
			}

			// Split collectors into two groups:
			//   background — safe to run concurrently while pterm spinner is active
			//   interactive — need exclusive terminal (kernel_modules TUI)
			allCols := defaultCollectors()
			var bgCols []collectors.Collector
			var tuiCols []collectors.Collector
			for _, col := range allCols {
				if col.Name() == collectors.KernelModsCollectorName {
					tuiCols = append(tuiCols, col)
				} else {
					bgCols = append(bgCols, col)
				}
			}

			// --- Phase 1: background collectors (with spinner) ---
			var snap *snapshot.Snapshot
			filteredBg := filterActive(bgCols, opts)
			if len(filteredBg) > 0 {
				pterm.Info.Println("Running collectors…")
				spin, _ := pterm.DefaultSpinner.Start("Collecting environment data")
				var bgErr error
				snap, bgErr = snapshot.Create(ctx, filteredBg, snapshot.Options{})
				spin.Stop()
				if bgErr != nil {
					pterm.Warning.Println("Some collectors returned errors:", bgErr)
				}
			} else {
				snap = snapshot.New()
			}

			// --- Phase 2: interactive collectors (after spinner, TUI can own terminal) ---
			if collectorActive(collectors.KernelModsCollectorName, opts) {
				for _, col := range tuiCols {
					sec, colErr := col.Collect(ctx)
					if colErr != nil {
						pterm.Warning.Printf("collector %q error: %v\n", col.Name(), colErr)
						continue
					}
					if applyErr := snapshot.ApplySection(snap, sec); applyErr != nil {
						pterm.Warning.Printf("apply %q: %v\n", col.Name(), applyErr)
					}
				}
			}

			if outputPath == "" {
				var pathErr error
				outputPath, pathErr = storage.DefaultPath()
				if pathErr != nil {
					return pathErr
				}
			}

			if err := storage.Save(outputPath, snap); err != nil {
				return fmt.Errorf("save snapshot: %w", err)
			}

			pterm.Success.Printf("Snapshot saved to %s\n", outputPath)
			printSnapshotSummary(snap)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "path to save the snapshot (default: ~/.envsnapshot/snapshot-<ts>.json)")
	cmd.Flags().StringVar(&include, "include", "", "comma-separated list of collectors to include (default: all)")
	cmd.Flags().StringVar(&exclude, "exclude", "", "comma-separated list of collectors to exclude")
	cmd.Flags().IntVar(&timeout, "timeout", 120, "timeout in seconds for all collectors")

	return cmd
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func printSnapshotSummary(snap *snapshot.Snapshot) {
	table := pterm.TableData{
		{"Section", "Status"},
	}
	mark := func(ok bool) string {
		if ok {
			return pterm.Green("✓")
		}
		return pterm.Yellow("–")
	}

	table = append(table, []string{"env", mark(len(snap.Sections.Env) > 0)})
	table = append(table, []string{"system", mark(snap.Sections.System != nil)})
	table = append(table, []string{"go", mark(snap.Sections.Go != nil)})
	table = append(table, []string{"packages", mark(len(snap.Sections.Packages) > 0)})
	table = append(table, []string{"kernel_modules", mark(len(snap.Sections.KernelMods) > 0)})
	table = append(table, []string{"repositories", mark(len(snap.Sections.Repositories) > 0)})

	_ = pterm.DefaultTable.WithHasHeader().WithData(table).Render()
}

// collectorActive reports whether a collector with the given name should run
// given the include/exclude options.
func collectorActive(name string, opts snapshot.Options) bool {
	if len(opts.Include) > 0 {
		for _, s := range opts.Include {
			if s == name {
				return true
			}
		}
		return false
	}
	for _, s := range opts.Exclude {
		if s == name {
			return false
		}
	}
	return true
}

// filterActive returns only those collectors that are active per opts.
func filterActive(cols []collectors.Collector, opts snapshot.Options) []collectors.Collector {
	var out []collectors.Collector
	for _, c := range cols {
		if collectorActive(c.Name(), opts) {
			out = append(out, c)
		}
	}
	return out
}
