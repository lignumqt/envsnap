// Package types defines shared data types used by both collectors and snapshot engine.
// It has no external dependencies within the module, preventing import cycles.
package types

// SystemInfo holds operating system and hardware details.
type SystemInfo struct {
	OS            string `json:"os"`
	OSVersion     string `json:"os_version"`
	KernelVersion string `json:"kernel_version"`
	Shell         string `json:"shell"`
	Arch          string `json:"arch"`
}

// GoInfo holds Go toolchain details.
type GoInfo struct {
	Version string            `json:"version"`
	Env     map[string]string `json:"env"`
}

// Package represents an installed system package.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Manager string `json:"manager"` // "apt" | "dnf"
}

// KernelModule represents a kernel module — either currently loaded, installed
// on disk, or both.
type KernelModule struct {
	Name      string   `json:"name"`
	Size      int64    `json:"size,omitempty"`
	UsedBy    string   `json:"used_by,omitempty"`
	Loaded    bool     `json:"loaded"`
	Installed bool     `json:"installed"`
	Depends   []string `json:"depends,omitempty"`
}

// Repository represents a configured package repository.
type Repository struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// DNFData is the combined result from the DNF packages collector.
type DNFData struct {
	Packages     []Package
	Repositories []Repository
}
