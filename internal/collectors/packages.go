package collectors

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/lignumqt/envsnap/internal/types"
)

// runCmd runs a command, captures stdout and stderr separately.
// On error it returns a wrapped error that includes the stderr output.
func runCmd(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return nil, fmt.Errorf("%w\nstderr: %s", err, stderrStr)
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

const PackagesCollectorName = "packages"

// PackagesCollector auto-detects whether the system uses APT or DNF and
// collects installed packages (and repositories for DNF).
type PackagesCollector struct{}

func NewPackagesCollector() *PackagesCollector { return &PackagesCollector{} }

func (c *PackagesCollector) Name() string { return PackagesCollectorName }

func (c *PackagesCollector) Collect(ctx context.Context) (Section, error) {
	// Prefer APT/dpkg if available.
	if path, err := exec.LookPath("dpkg-query"); err == nil {
		debugf(ctx, "package manager: APT/dpkg (%s)", path)
		return collectAPT(ctx, path)
	}
	// Try dnf5 first (Fedora 40+ ships dnf5 as a standalone binary).
	if path, err := exec.LookPath("dnf5"); err == nil {
		debugf(ctx, "package manager: DNF5 (%s)", path)
		return collectDNF(ctx, path, true)
	}
	if path, err := exec.LookPath("dnf"); err == nil {
		debugf(ctx, "package manager: DNF (%s)", path)
		return collectDNF(ctx, path, false)
	}
	return Section{Name: PackagesCollectorName, Data: []types.Package{}},
		fmt.Errorf("no supported package manager found (dpkg-query, dnf5, or dnf)")
}

// ── APT / dpkg ──────────────────────────────────────────────────────────────

func collectAPT(ctx context.Context, dpkgQuery string) (Section, error) {
	// dpkg-query -W -f='${Package} ${Version}\n'
	debugf(ctx, "running: %s -W -f=${Package} ${Version}\\n", dpkgQuery)
	out, err := runCmd(ctx, dpkgQuery, "-W", "-f=${Package} ${Version}\n")
	if err != nil {
		return Section{}, fmt.Errorf("dpkg-query failed: %w", err)
	}

	pkgs := parseAPTOutput(out)
	debugf(ctx, "dpkg-query: %d packages collected", len(pkgs))
	return Section{Name: PackagesCollectorName, Data: pkgs}, nil
}

func parseAPTOutput(data []byte) []types.Package {
	var pkgs []types.Package
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, " ", 2)
		pkgs = append(pkgs, types.Package{
			Name:    fields[0],
			Manager: "apt",
		})
		if len(fields) == 2 {
			pkgs[len(pkgs)-1].Version = strings.TrimSpace(fields[1])
		}
	}
	return pkgs
}

// ── DNF ─────────────────────────────────────────────────────────────────────

// collectDNF gathers packages and repositories using dnf or dnf5.
// isDNF5 switches to the dnf5 CLI syntax which differs from dnf4.
func collectDNF(ctx context.Context, dnfPath string, isDNF5 bool) (Section, error) {
	// Packages
	pkgs, err := dnfListInstalled(ctx, dnfPath, isDNF5)
	if err != nil && !isDNF5 {
		// On Fedora 40+ "dnf" may be a symlink to dnf5 — retry with dnf5 syntax.
		debugf(ctx, "dnf4 syntax failed, retrying with dnf5 syntax: %v", err)
		pkgs, err = dnfListInstalled(ctx, dnfPath, true)
	}
	if err != nil {
		return Section{}, err
	}

	// Repositories — non-fatal if they fail.
	repos, repoErr := dnfRepoList(ctx, dnfPath, isDNF5)
	if repoErr != nil && !isDNF5 {
		debugf(ctx, "dnf4 repolist failed, retrying with dnf5 syntax: %v", repoErr)
		repos, _ = dnfRepoList(ctx, dnfPath, true)
	} else if repoErr != nil {
		debugf(ctx, "repolist error (non-fatal): %v", repoErr)
	}

	return Section{
		Name: PackagesCollectorName,
		Data: types.DNFData{Packages: pkgs, Repositories: repos},
	}, nil
}

func dnfListInstalled(ctx context.Context, dnfPath string, isDNF5 bool) ([]types.Package, error) {
	// dnf5: dnf5 list --installed
	// dnf4: dnf  list installed -q
	var args []string
	if isDNF5 {
		args = []string{"list", "--installed"}
	} else {
		args = []string{"list", "installed", "-q"}
	}
	debugf(ctx, "running: %s %s", dnfPath, strings.Join(args, " "))

	out, err := runCmd(ctx, dnfPath, args...)
	if err != nil {
		return nil, fmt.Errorf("dnf list installed failed: %w", err)
	}
	pkgs := parseDNFList(out)
	debugf(ctx, "dnf list: %d packages collected", len(pkgs))
	return pkgs, nil
}

func parseDNFList(data []byte) []types.Package {
	var pkgs []types.Package
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "Installed") || strings.HasPrefix(line, "Last") {
			continue
		}
		// Fields: "name.arch  version-release  repo"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		nameArch := fields[0]
		// Strip arch suffix (last dot-separated segment)
		name := nameArch
		if idx := strings.LastIndexByte(nameArch, '.'); idx > 0 {
			name = nameArch[:idx]
		}

		version := fields[1]
		// Strip release tag (everything after the last '-') for cleaner output.
		if idx := strings.LastIndexByte(version, '-'); idx > 0 {
			version = version[:idx]
		}

		pkgs = append(pkgs, types.Package{
			Name:    name,
			Version: version,
			Manager: "dnf",
		})
	}
	return pkgs
}

func dnfRepoList(ctx context.Context, dnfPath string, isDNF5 bool) ([]types.Repository, error) {
	// dnf5: dnf5 repolist --all
	// dnf4: dnf  repolist all -q
	var args []string
	if isDNF5 {
		args = []string{"repolist", "--all"}
	} else {
		args = []string{"repolist", "all", "-q"}
	}
	debugf(ctx, "running: %s %s", dnfPath, strings.Join(args, " "))

	out, err := runCmd(ctx, dnfPath, args...)
	if err != nil {
		return nil, fmt.Errorf("dnf repolist failed: %w", err)
	}
	repos := parseDNFRepoList(out)
	debugf(ctx, "dnf repolist: %d repositories collected", len(repos))
	return repos, nil
}

func parseDNFRepoList(data []byte) []types.Repository {
	var repos []types.Repository
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "repo id") || strings.HasPrefix(line, "Last") {
			continue
		}
		// Fields: id  name  status (status may be "enabled" or "disabled")
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		id := fields[0]
		status := strings.ToLower(fields[len(fields)-1])
		// Name is everything between id and status
		name := strings.Join(fields[1:len(fields)-1], " ")
		if name == "" {
			name = id
		}

		repos = append(repos, types.Repository{
			ID:      id,
			Name:    name,
			Enabled: status == "enabled",
		})
	}
	return repos
}
