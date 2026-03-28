package restore

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/lignumqt/envsnap/internal/snapshot"
)

// ActionType describes what a restore action will do.
type ActionType string

const (
	ActionInstallPackage ActionType = "install_package"
	ActionKernelModule   ActionType = "kernel_module"
	ActionEnableRepo     ActionType = "enable_repo"
)

// RestoreAction represents a single step in the restore plan.
type RestoreAction struct {
	Type        ActionType
	Description string
	Command     string
	Args        []string
}

// Plan holds all restore actions derived from a snapshot.
type Plan struct {
	Actions []RestoreAction
	Manager string // "apt" | "dnf" | ""
}

// Build creates a restore plan from a snapshot.
func Build(snap *snapshot.Snapshot) (*Plan, error) {
	plan := &Plan{}

	if len(snap.Sections.Packages) > 0 {
		plan.Manager = snap.Sections.Packages[0].Manager
	}

	installed := currentlyInstalled(plan.Manager)

	for _, pkg := range snap.Sections.Packages {
		if installed[pkg.Name] {
			continue
		}
		action := RestoreAction{
			Type:        ActionInstallPackage,
			Description: fmt.Sprintf("Install package: %s (%s)", pkg.Name, pkg.Version),
		}
		switch plan.Manager {
		case "apt":
			action.Command = "apt-get"
			action.Args = []string{"install", "-y", pkg.Name}
		case "dnf":
			action.Command = "dnf"
			action.Args = []string{"install", "-y", pkg.Name}
		default:
			action.Command = "echo"
			action.Args = []string{"[unknown package manager]", pkg.Name}
		}
		plan.Actions = append(plan.Actions, action)
	}

	for _, mod := range snap.Sections.KernelMods {
		plan.Actions = append(plan.Actions, RestoreAction{
			Type:        ActionKernelModule,
			Description: fmt.Sprintf("Kernel module (manual): modprobe %s", mod.Name),
			Command:     "modprobe",
			Args:        []string{mod.Name},
		})
	}

	return plan, nil
}

// Apply executes (or prints, for dry-run) all actions in the plan.
func Apply(plan *Plan, dryRun bool) error {
	if len(plan.Actions) == 0 {
		fmt.Println("Nothing to restore — environment is already up to date.")
		return nil
	}

	if dryRun {
		fmt.Println("Dry-run mode — no changes will be made.")
		fmt.Println()
	}

	var errs []string
	for _, action := range plan.Actions {
		switch action.Type {
		case ActionKernelModule:
			if dryRun {
				fmt.Printf("  [kernel] %s\n", action.Description)
			} else {
				fmt.Printf("  [kernel] (skipped) %s  — run manually if needed\n", action.Description)
			}
			continue

		case ActionInstallPackage:
			full := action.Command + " " + strings.Join(action.Args, " ")
			if dryRun {
				fmt.Printf("  [install] %s\n", full)
				continue
			}
			fmt.Printf("  [install] Running: %s\n", full)
			if err := runInstall(action.Command, action.Args...); err != nil {
				errs = append(errs, fmt.Sprintf("failed to install %s: %v", action.Args[len(action.Args)-1], err))
			}

		default:
			fmt.Printf("  [info] %s\n", action.Description)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("restore completed with errors:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

func runInstall(command string, args ...string) error {
	var cmdArgs []string
	if os.Getuid() != 0 {
		cmdArgs = append(cmdArgs, "sudo", command)
	} else {
		cmdArgs = append(cmdArgs, command)
	}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...) //#nosec G204
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func currentlyInstalled(manager string) map[string]bool {
	installed := make(map[string]bool)
	var out []byte
	var err error

	switch manager {
	case "apt":
		out, err = exec.Command("dpkg-query", "-W", "-f=${Package}\n").Output()
	case "dnf":
		out, err = exec.Command("dnf", "list", "installed", "-q").Output()
	default:
		return installed
	}
	if err != nil {
		return installed
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name := strings.Fields(line)[0]
		if idx := strings.LastIndexByte(name, '.'); idx > 0 && manager == "dnf" {
			name = name[:idx]
		}
		installed[name] = true
	}
	return installed
}
