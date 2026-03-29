// Package export converts an envsnap Snapshot into a ready-to-use
// environment-reproduction artifact: a bash script, a Dockerfile, an Ansible
// playbook, or a plain shell env-variable fragment.
package export

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lignumqt/envsnap/internal/snapshot"
	"github.com/lignumqt/envsnap/internal/types"
)

// Format identifies the output format for export.
type Format string

const (
	FormatScript     Format = "script"
	FormatDockerfile Format = "dockerfile"
	FormatAnsible    Format = "ansible"
	FormatEnv        Format = "env"
)

// Options controls export behaviour.
type Options struct {
	Format       Format
	SkipPackages bool // omit package-installation section
	SkipEnv      bool // omit environment-variable section
	SkipModules  bool // omit kernel-module section
	OnlyLoaded   bool // kernel modules: include only loaded (not installed-only)
}

// Export renders snap into the requested artifact format and returns the
// resulting text. An error is returned only for unknown formats.
func Export(snap *snapshot.Snapshot, opts Options) (string, error) {
	if opts.Format == "" {
		opts.Format = FormatScript
	}
	switch opts.Format {
	case FormatScript:
		return renderScript(snap, opts)
	case FormatDockerfile:
		return renderDockerfile(snap, opts)
	case FormatAnsible:
		return renderAnsible(snap, opts)
	case FormatEnv:
		return renderEnv(snap, opts)
	default:
		return "", fmt.Errorf("unknown export format %q; choose: script, dockerfile, ansible, env", opts.Format)
	}
}

// ─── shared helpers ───────────────────────────────────────────────────────────

// importantEnvPrefixes mirrors the prefixes used in the diff engine so that
// script / dockerfile / ansible outputs stay consistent with diff output.
var importantEnvPrefixes = []string{
	"GO", "PATH", "HOME", "JAVA", "PYTHON", "NODE", "NVM", "CARGO", "RUST",
}

// containerSkipPrefixes: packages whose name starts with any of these prefixes
// are excluded from Dockerfile package layers.  These are host-identity,
// hardware-specific, or bootloader packages that either conflict with the
// container base image or serve no purpose inside a container.
var containerSkipPrefixes = []string{
	// Fedora / RHEL release identity — only *-container belongs in a container.
	"fedora-release-identity-",
	"fedora-release-server",
	"fedora-release-workstation",
	"fedora-release-iot",
	"fedora-release-kde",
	"fedora-release-silverblue",
	"fedora-release-kinoite",
	"centos-release-",
	"rhel-release",
	// systemd split variants — conflict with systemd-standalone-* in container base.
	"systemd-standalone-",
	// Ubuntu host-specific meta-packages.
	"ubuntu-advantage-tools",
	"ubuntu-pro-client",
	// Bootloader — meaningless in containers and may conflict.
	"grub-",    // Debian/Ubuntu grub packages
	"grub2-",   // Fedora/RHEL grub2 packages
	"dracut",   // initramfs builder: dracut, dracut-config-rescue
	"plymouth", // boot splash: plymouth, plymouth-core-libs, plymouth-scripts
	// Kernel packages — host-specific, cannot be loaded inside a container.
	"kernel",         // kernel, kernel-core, kernel-devel, kernel-modules, …
	"linux-image-",   // Debian/Ubuntu kernel images
	"linux-headers-", // Debian/Ubuntu kernel headers
	"linux-modules-", // Debian/Ubuntu kernel modules
	// Firmware — hardware drivers, irrelevant in containers.
	"linux-firmware", // linux-firmware, linux-firmware-whence
	"b43-",           // b43-fwcutter, b43-openfwwf
}

// containerSkipExact: exact package names excluded from Dockerfile layers.
var containerSkipExact = map[string]bool{
	// systemd-sysusers conflicts with systemd-standalone-sysusers present in
	// the Fedora container base image.
	"systemd-sysusers": true,
	"systemd-tmpfiles": true,
	// Bootloader helpers.
	"grubby":      true,
	"os-prober":   true,
	"kexec-tools": true,
	// CPU microcode — host hardware specific.
	"microcode_ctl": true,
}

// containerSkipSuffixes: packages whose name ends with any of these are excluded.
// This catches all *-firmware packages (amd-gpu-firmware, nvidia-gpu-firmware, …)
// and *-fwcutter helpers.
var containerSkipSuffixes = []string{
	"-firmware",
	"-fwcutter",
}

// isContainerSkippedPackage returns true for packages that should not be
// installed inside a Docker/OCI container image.
func isContainerSkippedPackage(name string) bool {
	if containerSkipExact[name] {
		return true
	}
	for _, prefix := range containerSkipPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	for _, suffix := range containerSkipSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// filteredEnvVars returns a copy of env with sensitive keys removed.
// When importantOnly is true only keys matching importantEnvPrefixes are kept.
func filteredEnvVars(env map[string]string, importantOnly bool) map[string]string {
	out := make(map[string]string)
	for k, v := range env {
		if types.IsSensitive(k) {
			continue
		}
		if importantOnly {
			found := false
			for _, p := range importantEnvPrefixes {
				if strings.HasPrefix(k, p) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		out[k] = v
	}
	return out
}

// sortedKeys returns the keys of m sorted alphabetically.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// activeMods returns the kernel modules from snap, optionally filtered to
// currently loaded ones only.
func activeMods(snap *snapshot.Snapshot, onlyLoaded bool) []types.KernelModule {
	if !onlyLoaded {
		return snap.Sections.KernelMods
	}
	out := make([]types.KernelModule, 0, len(snap.Sections.KernelMods))
	for _, m := range snap.Sections.KernelMods {
		if m.Loaded {
			out = append(out, m)
		}
	}
	return out
}

// dockerBaseImage maps SystemInfo OS fields to a Docker image reference.
func dockerBaseImage(sys *types.SystemInfo) string {
	if sys == nil {
		return "ubuntu:latest"
	}
	os := strings.ToLower(sys.OS)
	ver := sys.OSVersion
	switch {
	case strings.Contains(os, "ubuntu"):
		if ver != "" {
			return "ubuntu:" + ver
		}
		return "ubuntu:latest"
	case strings.Contains(os, "debian"):
		if ver != "" {
			return "debian:" + ver
		}
		return "debian:latest"
	case strings.Contains(os, "fedora"):
		if ver != "" {
			return "fedora:" + ver
		}
		return "fedora:latest"
	case strings.Contains(os, "centos"):
		return "centos:stream9"
	case strings.Contains(os, "red hat"), strings.Contains(os, "rhel"):
		return "redhat/ubi9:latest"
	default:
		return "ubuntu:latest"
	}
}

// packageManager returns the manager name detected from the snapshot packages.
func packageManager(snap *snapshot.Snapshot) string {
	if len(snap.Sections.Packages) > 0 {
		return snap.Sections.Packages[0].Manager
	}
	return ""
}

// goVersion returns the Go version string stored in the snapshot (e.g. "go1.26.1").
func goVersion(snap *snapshot.Snapshot) string {
	if snap.Sections.Go != nil {
		return snap.Sections.Go.Version
	}
	return ""
}

// goArch returns the CPU architecture from the snapshot (defaults to "amd64").
func goArch(snap *snapshot.Snapshot) string {
	if snap.Sections.System != nil && snap.Sections.System.Arch != "" {
		return snap.Sections.System.Arch
	}
	return "amd64"
}

// shellQuote wraps s in single quotes, correctly escaping embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// dockerfileQuote wraps s in double quotes, escaping \, " and $ for Dockerfile ENV.
func dockerfileQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `$`, `\$`)
	return `"` + s + `"`
}

// yamlQuote wraps s in double quotes, escaping \ and " for YAML.
func yamlQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
