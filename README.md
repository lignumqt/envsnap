# envsnap

[![Go Reference](https://pkg.go.dev/badge/github.com/lignumqt/envsnap.svg)](https://pkg.go.dev/github.com/lignumqt/envsnap)
[![Go Report Card](https://goreportcard.com/badge/github.com/lignumqt/envsnap)](https://goreportcard.com/report/github.com/lignumqt/envsnap)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Package envsnap captures a structured snapshot of a Linux development environment,
compares snapshots across machines, and restores missing packages.

## Features

- **snapshot** — capture your current environment (packages, env vars, system info, Go toolchain, kernel modules); interactive TUI lets you deselect packages and kernel modules before saving
- **inspect** — pretty-print the contents of a saved snapshot
- **diff** — compare two snapshots with color-coded output (added / removed / changed)
- **restore** — dry-run or apply installation of packages missing on the current machine
- **export** — convert a snapshot into a ready-to-use reproduction artifact: bash script, Dockerfile, Ansible playbook, or shell env fragment

## Supported package managers

| Manager   | Distros              | What is collected                     |
|-----------|----------------------|---------------------------------------|
| APT/dpkg  | Ubuntu, Debian       | Installed packages                    |
| DNF/DNF5  | Fedora, RHEL, CentOS | Installed packages + enabled repos    |

## Installation

```bash
go install github.com/lignumqt/envsnap/cmd/envsnap@latest
```

> **Note:** version, commit hash and build date are injected via `-ldflags` at build time.
> `go install …@latest` produces a binary with these fields set to `dev`/`none`.
> Use `make build` for proper version stamping.

Build from source:

```bash
# Build binary to ./envsnap, then install to /usr/local/bin
make build
sudo make install
```

## Getting started

```bash
# Take a full snapshot (kernel module TUI opens automatically)
envsnap snapshot

# Save to a specific file, include only selected collectors
envsnap snapshot -o ~/backups/workstation.json --include env,system,go,packages

# Skip kernel modules entirely (no interactive TUI)
envsnap snapshot --exclude kernel_modules

# Inspect a snapshot (metadata, system, Go, first 20 packages, kernel modules)
envsnap inspect ~/.envsnapshot/snapshot-2026-03-29T10-00-00Z.json

# Browse all captured packages in a scrollable, filterable full-screen TUI
envsnap inspect snapshot.json --packages

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

# Export snapshot as a bash setup script (stdout)
envsnap export snapshot.json

# Export as Dockerfile, write to file
envsnap export snapshot.json -f dockerfile -o Dockerfile

# Export as Ansible playbook
envsnap export snapshot.json -f ansible -o playbook.yml

# Inject captured env vars into the current shell session
source <(envsnap export snapshot.json -f env)
```

## Usage

### `snapshot`

Captures the current environment and writes a JSON file.

| Flag              | Short | Default                          | Description                                              |
|-------------------|-------|----------------------------------|----------------------------------------------------------|
| `--output`        | `-o`  | `~/.envsnapshot/snapshot-*.json` | Path to write the snapshot to                            |
| `--include`       |       | *(all collectors)*               | Comma-separated list of collectors to run                |
| `--exclude`       |       | *(none)*                         | Comma-separated list of collectors to skip               |

When `kernel_modules` or `packages` is included (the default), background collectors run first with a progress spinner, then the interactive TUIs launch in sequence:

1. **Package selector** — all installed packages are pre-selected; deselect anything you don't want in the snapshot. Use `Space` to toggle, `a` to toggle all visible, `/` to filter, `Enter` to confirm.
2. **Kernel module selector** — same controls; choose which modules to record.

Both TUIs are skipped automatically when stdin is not a terminal (pipe, CI, `--non-interactive`).

### `inspect`

| Flag          | Default | Description                                                              |
|---------------|---------|--------------------------------------------------------------------------|
| `--env`       | false   | Print all captured environment variables (sorted)                        |
| `--packages`  | false   | Open a full-screen interactive browser for all captured packages         |

`inspect` shows only the first 20 packages by default to avoid flooding the terminal; the remaining packages are summarised as `… and N more packages. Use --packages to browse all N packages interactively.`

The `--packages` browser supports:
- `↑ / ↓ / j / k` — scroll line by line
- `PgUp / PgDn` — scroll page by page
- `g / G` — jump to top / bottom
- `/` — enter filter mode (searches name and version)
- `Esc` — clear active filter (or quit if no filter)
- `q / Ctrl+C` — quit

When stdout is not a terminal (pipe, redirect), `--packages` falls back to printing the full plain table.

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

### `export`

Converts a snapshot into a text artifact for reproducing the environment on another machine or in a container. **Nothing is executed automatically** — the command writes text to stdout (or to a file with `-o`) and you use the artifact yourself.

| Flag               | Short | Default    | Description                                                         |
|--------------------|-------|------------|---------------------------------------------------------------------|
| `--format`         | `-f`  | `script`   | Output format: `script`, `dockerfile`, `ansible`, `env`             |
| `--output`         | `-o`  | *(stdout)* | Write output to a file instead of stdout                            |
| `--skip-packages`  |       | false      | Omit the package-installation section                               |
| `--skip-env`       |       | false      | Omit the environment-variables section                              |
| `--skip-modules`   |       | false      | Omit the kernel-modules section                                     |
| `--only-loaded`    |       | false      | Include only currently loaded kernel modules (skip installed-only)  |

#### Format details

**`script`** (default) — a `bash` script with `set -euo pipefail`:
- `apt-get install` or `dnf install` for all captured packages
- Downloads the exact Go version from `go.dev/dl` and extracts to `/usr/local/go`
- `sudo modprobe <name>` for each kernel module + writes `/etc/modules-load.d/envsnap.conf` to persist them across reboots
- `export KEY=VALUE` for important, non-sensitive environment variables

```bash
envsnap export snap.json -o setup.sh
bash setup.sh
```

**`dockerfile`** — a `Dockerfile` text file (does **not** build an image):
- `FROM ubuntu:VERSION` / `FROM fedora:VERSION` based on captured system info
- `RUN apt-get install` / `RUN dnf install` layer
- `RUN wget … && tar …` layer to install the exact Go version
- `ENV KEY=VALUE` for important, non-sensitive variables
- Kernel modules **cannot** run inside a container (they operate at the host kernel level), so they appear as a documented comment block with `modprobe` commands intended for the host machine

```bash
envsnap export snap.json -f dockerfile -o Dockerfile
docker build -t my-env .
docker run --rm -it my-env bash
```

**`ansible`** — an Ansible playbook YAML (`become: true`, targets `localhost` by default):
- `ansible.builtin.apt` / `ansible.builtin.dnf` task with a loop over all packages
- `ansible.builtin.get_url` + `ansible.builtin.unarchive` block for Go
- `community.general.modprobe` task with `ignore_errors: true` — idempotent module loading (unlike raw `modprobe`), runs on the managed host (not inside a container)
- `ansible.builtin.copy` task that writes `/etc/modules-load.d/envsnap.conf` to persist modules across reboots
- `ansible.builtin.copy` task that writes env vars to `/etc/profile.d/envsnap-env.sh`
- Requires: `ansible-galaxy collection install community.general`

```bash
envsnap export snap.json -f ansible -o playbook.yml
# Run locally:
ansible-playbook playbook.yml
# Run against a remote host:
ansible-playbook playbook.yml -e target=myserver
```

**`env`** — a shell env fragment, all non-sensitive variables (no important-prefix filter):

```bash
# Apply to the current shell session:
source <(envsnap export snap.json -f env)

# Or eval form:
eval "$(envsnap export snap.json -f env)"
```

#### Sensitive variable filtering

All export formats automatically filter out variables whose names contain: `AWS_`, `GITHUB_TOKEN`, `SECRET`, `PASSWORD`, `TOKEN`, `KEY`, `PASSWD`. These are never written to any output.

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

## Snapshot format (JSON schema)

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

## Examples

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

### Reproduce environment in a Docker container

```bash
# On the source machine
envsnap snapshot -o ~/workstation.json

# Generate a Dockerfile (skip packages for a leaner image — add only what you need)
envsnap export ~/workstation.json -f dockerfile --skip-packages -o Dockerfile

# Build and run
docker build -t my-env .
docker run --rm -it my-env bash
```

```bash
# Pipe Dockerfile directly into docker build without writing it to disk
envsnap export ~/workstation.json -f dockerfile --skip-packages | docker build -t my-env -

# Cross-build for another architecture
envsnap export ~/workstation.json -f dockerfile --skip-packages \
  | docker buildx build --platform linux/arm64 -t my-env-arm64 -
```

> **Kernel modules and containers:** kernel modules cannot be loaded inside a container
> — they control the host kernel. All modules appear as a comment block with `modprobe`
> commands intended to be run on the host machine after the container is started.

### Automate environment setup with Ansible

```bash
# Generate playbook (only loaded modules, skip large package list)
envsnap export ~/workstation.json -f ansible --only-loaded --skip-packages -o playbook.yml
ansible-galaxy collection install community.general
```

**Run against localhost (the current machine):**
```bash
ansible-playbook playbook.yml
```

**Preview what would change without applying anything:**
```bash
ansible-playbook playbook.yml --check
```

**Run against a single remote host:**
```bash
ansible-playbook playbook.yml -e target=storage-server -i inventory.ini
```

**Run against an inventory group:**
```ini
# inventory.ini
[storage]
storage01 ansible_user=root
storage02 ansible_user=root
```

```bash
ansible-playbook playbook.yml -e target=storage -i inventory.ini
```

**Limit to a subset of hosts with `--limit`:**
```bash
ansible-playbook playbook.yml -i inventory.ini -l storage01,storage02
```

**Ask for sudo password (no passwordless sudo):**
```bash
ansible-playbook playbook.yml -K -i inventory.ini
```

**Use a specific SSH key:**
```bash
ansible-playbook playbook.yml --private-key ~/.ssh/id_storage -i inventory.ini
```

### Export env vars into a CI/CD pipeline

```bash
# Inject environment variables from a stored baseline snapshot into a CI job
source <(envsnap export baseline.json -f env --skip-packages --skip-modules)
```

Write to a `.env` file for Docker Compose or tools that read it:
```bash
envsnap export baseline.json -f env --skip-packages --skip-modules \
  | grep -v '^#' > .env       # strip comment lines, keep KEY=VALUE pairs
```

Use in a Makefile target:
```makefile
env: baseline.json
	envsnap export $< -f env --skip-packages --skip-modules -o .env
```

### One-line environment clone to current shell

```bash
# Grab a colleague's snapshot, inject their env vars immediately
curl -s https://example.com/colleague-snap.json -o /tmp/snap.json
source <(envsnap export /tmp/snap.json -f env)
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
