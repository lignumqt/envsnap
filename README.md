# EnvSnapshot

A CLI tool for capturing a structured snapshot of your Linux development environment, comparing snapshots across machines, and restoring missing packages.

## Features

- **snapshot** — capture your current environment (packages, env vars, system info, Go toolchain, kernel modules)
- **inspect** — pretty-print the contents of a saved snapshot
- **diff** — compare two snapshots with color-coded output (added / removed / changed)
- **restore** — dry-run or apply installation of packages missing on the current machine

## Supported package managers

| Manager   | Distros              | What is collected                     |
|-----------|----------------------|---------------------------------------|
| APT/dpkg  | Ubuntu, Debian       | Installed packages                    |
| DNF/DNF5  | Fedora, RHEL, CentOS | Installed packages + enabled repos    |

## Installation

```bash
# Build binary to ./envsnap, then install to /usr/local/bin
make build
sudo make install
```

Or install directly with Go:

```bash
go install github.com/lignumqt/envsnap/cmd/envsnap@latest
```

> **Note:** version, commit hash and build date are injected via `-ldflags` at build time.
> `go install …@latest` produces a binary with these fields set to `dev`/`none`.
> Use `make build` to get proper version stamping.

## Quick start

```bash
# Take a full snapshot (kernel module TUI opens automatically)
envsnap snapshot

# Save to a specific file, include only selected collectors
envsnap snapshot -o ~/backups/workstation.json --include env,system,go,packages

# Skip kernel modules entirely (no interactive TUI)
envsnap snapshot --exclude kernel_modules

# Inspect a snapshot (metadata, system, Go, first 20 packages, kernel modules)
envsnap inspect ~/.envsnapshot/snapshot-2026-03-29T10-00-00Z.json

# Also print all captured environment variables
envsnap inspect snapshot.json --env

# Compare two snapshots
envsnap diff snapshot-old.json snapshot-new.json

# Show all differences without the default 50-entry limit
envsnap diff snapshot-old.json snapshot-new.json --limit 0

# Preview what restore would do (no changes made)
envsnap restore snapshot.json --dry-run

# Actually install missing packages (uses sudo internally)
envsnap restore snapshot.json --yes

# Print version, commit and build date
envsnap --version

# Enable verbose debug output globally (any subcommand)
envsnap --debug snapshot
```

## Command reference

### `snapshot`

Captures the current environment and writes a JSON file.

| Flag              | Short | Default                          | Description                                              |
|-------------------|-------|----------------------------------|----------------------------------------------------------|
| `--output`        | `-o`  | `~/.envsnapshot/snapshot-*.json` | Path to write the snapshot to                            |
| `--include`       |       | *(all collectors)*               | Comma-separated list of collectors to run                |
| `--exclude`       |       | *(none)*                         | Comma-separated list of collectors to skip               |

When `kernel_modules` is included (the default), background collectors run first with a progress spinner, then the kernel module TUI launches in the foreground.

### `inspect`

| Flag    | Default | Description                                         |
|---------|---------|-----------------------------------------------------|
| `--env` | false   | Print all captured environment variables (sorted)  |

`inspect` shows only the first 20 packages to avoid flooding the terminal; the rest are summarised as `… and N more packages`.

### `diff`

| Flag      | Default | Description                                          |
|-----------|---------|------------------------------------------------------|
| `--limit` | 50      | Max entries to display per category; `0` = unlimited |

Output is colour-coded: green = added, red = removed, cyan = changed, yellow = warnings.

### `restore`

| Flag        | Short | Default | Description                                             |
|-------------|-------|---------|---------------------------------------------------------|
| `--dry-run` |       | false   | Show what would be installed without making changes     |
| `--yes`     | `-y`  | false   | Apply the restore plan (installs missing packages)      |

If neither `--dry-run` nor `--yes` is provided, restore defaults to dry-run mode and prints a notice.
Package installation is executed with `sudo`; make sure your user has the necessary privileges.

### Global flags

| Flag      | Short | Description                                              |
|-----------|-------|----------------------------------------------------------|
| `--debug` | `-d`  | Print executed commands, captured stderr, and warnings   |

## Collectors

| Name             | What is collected                                                         |
|------------------|---------------------------------------------------------------------------|
| `env`            | All environment variables (`os.Environ()`)                                |
| `system`         | OS name/version, kernel version, shell, CPU architecture                  |
| `go`             | Go version, `GOROOT`, `GOPATH`, `GOMODCACHE`, `GOPROXY`, `GOARCH`, `GOOS` |
| `packages`       | Installed packages (APT or DNF) + enabled repositories                    |
| `kernel_modules` | Interactively-selected kernel modules (loaded + installed)                |

## Kernel module TUI

When `kernel_modules` is collected, an interactive selector opens. It shows modules from both `/proc/modules` (currently loaded) and `/lib/modules/<kernel-version>/` (installed on disk).

### Keyboard shortcuts

| Key                    | Action                                                                   |
|------------------------|--------------------------------------------------------------------------|
| `↑` / `k`              | Move cursor up                                                           |
| `↓` / `j`              | Move cursor down                                                         |
| `Space`                | Toggle selection of the current module                                   |
| `a`                    | Toggle selection of all currently visible modules                        |
| `i`                    | Toggle view: **loaded-only** ↔ **loaded + all installed**                |
| `/`                    | Enter filter mode — type to search by module name                        |
| `Esc` (in filter)      | Exit filter mode; the search string is **kept**                          |
| `Esc` (outside filter) | Clear the current filter string                                          |
| `Enter`                | Confirm save — shows confirmation dialog                                 |
| `Ctrl+C`               | Confirm cancel — shows confirmation dialog (selection will be discarded) |

#### Confirmation dialog

Pressing `Enter` or `Ctrl+C` opens a confirmation overlay instead of quitting immediately:

| Key           | Action                                |
|---------------|---------------------------------------|
| `y` / `Enter` | Confirm (save selection or discard)   |
| `n` / `Esc`   | Dismiss the dialog and go back        |

#### Non-obvious behaviors

- **Dependency auto-selection**: selecting a module automatically selects all its dependencies (read from `modules.dep`). Deselecting a module does **not** cascade — dependencies remain selected.
- **Filter is persistent**: exiting filter mode with `Esc` or `/` keeps the typed search string active. The list continues to show only matching modules. Press `Esc` again (outside filter mode) to clear it.
- **`i` toggles the module source**: by default both loaded (`/proc/modules`) and installed (`/lib/modules/`) modules are shown. Pressing `i` switches to loaded-only view; filter and selection are preserved.

## Snapshot storage

Snapshots are saved to `~/.envsnapshot/` by default, with filenames like `snapshot-2026-03-29T10-00-00Z.json`.
The directory is created with permissions `0700` and each file is written with `0600` (owner-readable only).

## Snapshot format

```json
{
  "meta": {
    "schema_version": "1.0",
    "created_at": "2026-03-29T10:00:00Z",
    "hostname": "workstation",
    "user": "alice",
    "tool_version": "v0.3.1-abc1234"
  },
  "sections": {
    "env": { "PATH": "/usr/local/bin:/usr/bin:…", "EDITOR": "nvim" },
    "system": {
      "os": "Ubuntu 24.04",
      "os_version": "24.04",
      "kernel_version": "6.8.0-51-generic",
      "shell": "/bin/bash",
      "arch": "amd64"
    },
    "go": {
      "version": "go1.26.1",
      "env": { "GOPATH": "/home/alice/go", "GOROOT": "/usr/local/go", "GOPROXY": "https://proxy.golang.org,direct" }
    },
    "packages": [
      { "name": "git", "version": "1:2.43.0-1ubuntu7.2", "manager": "apt" }
    ],
    "repositories": [
      { "id": "fedora", "name": "Fedora 41 — x86_64", "enabled": true }
    ],
    "kernel_modules": [
      { "name": "kvm_intel", "size": 458752, "used_by": "", "loaded": true, "installed": true, "depends": ["kvm"] }
    ]
  }
}
```

## Practical workflows

### Replicate a dev machine

```bash
# On the source machine
envsnap snapshot -o ~/workstation.json

# Copy the file to the target machine, then
envsnap restore ~/workstation.json --dry-run   # review the plan
envsnap restore ~/workstation.json --yes        # apply it
```

### Track environment drift over time

```bash
# Take a baseline
envsnap snapshot -o ~/snap-baseline.json --exclude kernel_modules

# After a week, take another
envsnap snapshot -o ~/snap-week1.json --exclude kernel_modules

# See what changed
envsnap diff ~/snap-baseline.json ~/snap-week1.json
```

### Capture only packages (CI / automation)

```bash
# Non-interactive — skip TUI, capture packages only
envsnap snapshot --include packages -o /tmp/ci-packages.json
```

### Audit installed packages without a full snapshot

```bash
envsnap snapshot --include packages -o /tmp/pkgs.json
envsnap inspect /tmp/pkgs.json
```

## Development

```bash
make build        # Build binary to ./envsnap
make install      # Install to /usr/local/bin
make fmt          # Format with gofmt -s
make vet          # Run go vet
make lint         # Run golangci-lint
make test         # Run tests with -race
make test-cover   # Tests + HTML coverage report (coverage.html)
make deps         # go mod tidy && go mod download
make update-deps  # go get -u ./... && go mod tidy
make clean        # Remove build artifacts and test cache
```

Run `make help` to see all available targets with descriptions.

## License

MIT
