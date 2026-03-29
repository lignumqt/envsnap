package diff

import (
	"fmt"

	"github.com/lignumqt/envsnap/internal/snapshot"
	"github.com/lignumqt/envsnap/internal/types"
)

// DiffResult holds the human-readable diff between two snapshots.
type DiffResult struct {
	Warnings []string
	Added    []string
	Removed  []string
	Changed  []string
}

// IsClean returns true when there are no differences.
func (d *DiffResult) IsClean() bool {
	return len(d.Warnings) == 0 && len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}

// Diff computes a structured diff between snapshot a (old) and b (new).
func Diff(a, b *snapshot.Snapshot) DiffResult {
	var res DiffResult
	diffSystem(&res, a.Sections.System, b.Sections.System)
	diffGo(&res, a.Sections.Go, b.Sections.Go)
	diffPackages(&res, a.Sections.Packages, b.Sections.Packages)
	diffRepos(&res, a.Sections.Repositories, b.Sections.Repositories)
	diffKernelMods(&res, a.Sections.KernelMods, b.Sections.KernelMods)
	diffEnv(&res, a.Sections.Env, b.Sections.Env)
	return res
}

func diffSystem(res *DiffResult, a, b *types.SystemInfo) {
	if a == nil && b == nil {
		return
	}
	if a == nil {
		res.Added = append(res.Added, "system: section added")
		return
	}
	if b == nil {
		res.Removed = append(res.Removed, "system: section removed")
		return
	}
	warn := func(field, va, vb string) {
		if va != vb {
			res.Warnings = append(res.Warnings, fmt.Sprintf("warning system.%s: %q to %q", field, va, vb))
		}
	}
	warn("os", a.OS, b.OS)
	warn("os_version", a.OSVersion, b.OSVersion)
	warn("kernel", a.KernelVersion, b.KernelVersion)
	warn("shell", a.Shell, b.Shell)
	warn("arch", a.Arch, b.Arch)
}

func diffGo(res *DiffResult, a, b *types.GoInfo) {
	if a == nil && b == nil {
		return
	}
	if a == nil {
		res.Added = append(res.Added, "go: toolchain added")
		return
	}
	if b == nil {
		res.Removed = append(res.Removed, "go: toolchain removed")
		return
	}
	if a.Version != b.Version {
		res.Warnings = append(res.Warnings, fmt.Sprintf("warning go version: %s to %s", a.Version, b.Version))
	}
	for _, key := range []string{"GOPATH", "GOROOT", "GOMODCACHE", "GOPROXY"} {
		va := a.Env[key]
		vb := b.Env[key]
		if va != vb {
			res.Changed = append(res.Changed, fmt.Sprintf("~ go.env.%s: %q to %q", key, va, vb))
		}
	}
}

func diffPackages(res *DiffResult, a, b []types.Package) {
	mapA := pkgMap(a)
	mapB := pkgMap(b)
	for name, verA := range mapA {
		if verB, ok := mapB[name]; !ok {
			res.Removed = append(res.Removed, fmt.Sprintf("- package: %s (%s)", name, verA))
		} else if verA != verB {
			res.Changed = append(res.Changed, fmt.Sprintf("~ package: %s %s to %s", name, verA, verB))
		}
	}
	for name, verB := range mapB {
		if _, ok := mapA[name]; !ok {
			res.Added = append(res.Added, fmt.Sprintf("+ package: %s (%s)", name, verB))
		}
	}
}

func pkgMap(pkgs []types.Package) map[string]string {
	m := make(map[string]string, len(pkgs))
	for _, p := range pkgs {
		m[p.Name] = p.Version
	}
	return m
}

func diffRepos(res *DiffResult, a, b []types.Repository) {
	mapA := repoMap(a)
	mapB := repoMap(b)
	for id, repA := range mapA {
		repB, ok := mapB[id]
		if !ok {
			res.Removed = append(res.Removed, fmt.Sprintf("- repo: %s (%s)", id, repA.Name))
			continue
		}
		if repA.Enabled != repB.Enabled {
			state := "disabled"
			if repB.Enabled {
				state = "enabled"
			}
			res.Changed = append(res.Changed, fmt.Sprintf("~ repo: %s now %s", id, state))
		}
	}
	for id, repB := range mapB {
		if _, ok := mapA[id]; !ok {
			res.Added = append(res.Added, fmt.Sprintf("+ repo: %s (%s)", id, repB.Name))
		}
	}
}

func repoMap(repos []types.Repository) map[string]types.Repository {
	m := make(map[string]types.Repository, len(repos))
	for _, r := range repos {
		m[r.ID] = r
	}
	return m
}

func diffKernelMods(res *DiffResult, a, b []types.KernelModule) {
	setA := modSet(a)
	setB := modSet(b)
	for name := range setA {
		if !setB[name] {
			res.Removed = append(res.Removed, fmt.Sprintf("- kernel_module: %s", name))
		}
	}
	for name := range setB {
		if !setA[name] {
			res.Added = append(res.Added, fmt.Sprintf("+ kernel_module: %s", name))
		}
	}
}

func modSet(mods []types.KernelModule) map[string]bool {
	m := make(map[string]bool, len(mods))
	for _, mod := range mods {
		m[mod.Name] = true
	}
	return m
}

func diffEnv(res *DiffResult, a, b map[string]string) {
	importantPrefixes := []string{"GO", "PATH", "HOME", "JAVA", "PYTHON", "NODE", "NVM", "CARGO", "RUST"}
	isImportant := func(key string) bool {
		for _, p := range importantPrefixes {
			if len(key) >= len(p) && key[:len(p)] == p {
				return true
			}
		}
		return false
	}
	for key, valA := range a {
		if !isImportant(key) || types.IsSensitive(key) {
			continue
		}
		valB, ok := b[key]
		if !ok {
			res.Removed = append(res.Removed, fmt.Sprintf("- env: %s", key))
		} else if valA != valB {
			res.Changed = append(res.Changed, fmt.Sprintf("~ env: %s changed", key))
		}
	}
	for key := range b {
		if !isImportant(key) || types.IsSensitive(key) {
			continue
		}
		if _, ok := a[key]; !ok {
			res.Added = append(res.Added, fmt.Sprintf("+ env: %s", key))
		}
	}
}
