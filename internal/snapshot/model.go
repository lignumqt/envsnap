package snapshot

import (
	"time"

	"github.com/lignumqt/envsnap/internal/types"
)

const SchemaVersion = "1.0"

// Snapshot is the root data structure for a captured environment state.
type Snapshot struct {
	Meta     Meta     `json:"meta"`
	Sections Sections `json:"sections"`
}

// Meta holds identifying information about a snapshot.
type Meta struct {
	SchemaVersion string    `json:"schema_version"`
	CreatedAt     time.Time `json:"created_at"`
	Hostname      string    `json:"hostname"`
	User          string    `json:"user"`
	ToolVersion   string    `json:"tool_version"`
}

// Sections holds all collected data; each field may be nil/empty if the collector
// was not executed or was excluded.
type Sections struct {
	Env          map[string]string    `json:"env,omitempty"`
	System       *types.SystemInfo    `json:"system,omitempty"`
	Go           *types.GoInfo        `json:"go,omitempty"`
	Packages     []types.Package      `json:"packages,omitempty"`
	KernelMods   []types.KernelModule `json:"kernel_modules,omitempty"`
	Repositories []types.Repository   `json:"repositories,omitempty"`
}
