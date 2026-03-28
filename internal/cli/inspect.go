package cli

import (
	"fmt"
	"sort"

	"github.com/lignumqt/envsnap/internal/snapshot"
	"github.com/lignumqt/envsnap/internal/storage"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func newInspectCmd() *cobra.Command {
	var showEnv bool

	cmd := &cobra.Command{
		Use:   "inspect <snapshot>",
		Short: "Display the contents of a snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			snap, err := storage.Load(args[0])
			if err != nil {
				return err
			}
			printInspect(snap, showEnv)
			return nil
		},
	}

	cmd.Flags().BoolVar(&showEnv, "env", false, "also print all environment variables")
	return cmd
}

func printInspect(snap *snapshot.Snapshot, showEnv bool) {
	// ── Meta ────────────────────────────────────────────────────────────────
	pterm.DefaultHeader.WithFullWidth().Println("Snapshot Metadata")

	metaTable := pterm.TableData{
		{"Field", "Value"},
		{"Schema", snap.Meta.SchemaVersion},
		{"Created", snap.Meta.CreatedAt.Format("2006-01-02 15:04:05 UTC")},
		{"Host", snap.Meta.Hostname},
		{"User", snap.Meta.User},
		{"Tool", snap.Meta.ToolVersion},
	}
	_ = pterm.DefaultTable.WithHasHeader().WithData(metaTable).Render()

	// ── System ──────────────────────────────────────────────────────────────
	if s := snap.Sections.System; s != nil {
		pterm.DefaultSection.Println("System")
		sysTable := pterm.TableData{
			{"Field", "Value"},
			{"OS", s.OS},
			{"OS Version", s.OSVersion},
			{"Kernel", s.KernelVersion},
			{"Shell", s.Shell},
			{"Arch", s.Arch},
		}
		_ = pterm.DefaultTable.WithHasHeader().WithData(sysTable).Render()
	}

	// ── Go ──────────────────────────────────────────────────────────────────
	if g := snap.Sections.Go; g != nil {
		pterm.DefaultSection.Println("Go Toolchain")
		goTable := pterm.TableData{{"Field", "Value"}, {"Version", g.Version}}
		for _, key := range []string{"GOROOT", "GOPATH", "GOMODCACHE", "GOPROXY", "GOARCH", "GOOS"} {
			if v, ok := g.Env[key]; ok {
				goTable = append(goTable, []string{key, v})
			}
		}
		_ = pterm.DefaultTable.WithHasHeader().WithData(goTable).Render()
	}

	// ── Packages ────────────────────────────────────────────────────────────
	if pkgs := snap.Sections.Packages; len(pkgs) > 0 {
		manager := pkgs[0].Manager
		pterm.DefaultSection.Printf("Packages (%s) — %d total\n", manager, len(pkgs))
		// Show only first 20 to avoid flooding the terminal.
		limit := 20
		if len(pkgs) < limit {
			limit = len(pkgs)
		}
		pkgTable := pterm.TableData{{"Name", "Version", "Manager"}}
		for _, p := range pkgs[:limit] {
			pkgTable = append(pkgTable, []string{p.Name, p.Version, p.Manager})
		}
		_ = pterm.DefaultTable.WithHasHeader().WithData(pkgTable).Render()
		if len(pkgs) > 20 {
			pterm.Info.Printf("… and %d more packages.\n", len(pkgs)-20)
		}
	}

	// ── Repositories ────────────────────────────────────────────────────────
	if repos := snap.Sections.Repositories; len(repos) > 0 {
		pterm.DefaultSection.Printf("Repositories — %d total\n", len(repos))
		repoTable := pterm.TableData{{"ID", "Name", "Enabled"}}
		for _, r := range repos {
			enabled := pterm.Green("yes")
			if !r.Enabled {
				enabled = pterm.Red("no")
			}
			repoTable = append(repoTable, []string{r.ID, r.Name, enabled})
		}
		_ = pterm.DefaultTable.WithHasHeader().WithData(repoTable).Render()
	}

	// ── Kernel Modules ──────────────────────────────────────────────────────
	if mods := snap.Sections.KernelMods; len(mods) > 0 {
		pterm.DefaultSection.Printf("Kernel Modules — %d selected\n", len(mods))
		modTable := pterm.TableData{{"Name", "Size (bytes)", "Used By"}}
		for _, m := range mods {
			modTable = append(modTable, []string{m.Name, fmt.Sprintf("%d", m.Size), m.UsedBy})
		}
		_ = pterm.DefaultTable.WithHasHeader().WithData(modTable).Render()
	}

	// ── Env (optional) ──────────────────────────────────────────────────────
	if showEnv && len(snap.Sections.Env) > 0 {
		pterm.DefaultSection.Printf("Environment Variables — %d vars\n", len(snap.Sections.Env))
		keys := make([]string, 0, len(snap.Sections.Env))
		for k := range snap.Sections.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		envTable := pterm.TableData{{"Variable", "Value"}}
		for _, k := range keys {
			val := snap.Sections.Env[k]
			if len(val) > 80 {
				val = val[:77] + "…"
			}
			envTable = append(envTable, []string{k, val})
		}
		_ = pterm.DefaultTable.WithHasHeader().WithData(envTable).Render()
	} else if !showEnv && len(snap.Sections.Env) > 0 {
		pterm.Info.Printf("Environment: %d variables captured (use --env to display)\n", len(snap.Sections.Env))
	}
}
