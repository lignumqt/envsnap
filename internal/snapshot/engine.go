package snapshot

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"sync"
	"time"

	"github.com/lignumqt/envsnap/internal/collectors"
	"github.com/lignumqt/envsnap/internal/types"
	"github.com/lignumqt/envsnap/internal/version"
)

// Options controls which collectors are run.
type Options struct {
	Include []string
	Exclude []string
}

// Create runs the provided collectors concurrently and assembles a Snapshot.
func Create(ctx context.Context, cols []collectors.Collector, opts Options) (*Snapshot, error) {
	active := filter(cols, opts)
	if len(active) == 0 {
		return nil, fmt.Errorf("no collectors selected")
	}

	type result struct {
		sec collectors.Section
		err error
	}

	results := make([]result, len(active))
	var wg sync.WaitGroup
	wg.Add(len(active))

	for i, col := range active {
		i, col := i, col
		go func() {
			defer wg.Done()
			sec, err := col.Collect(ctx)
			results[i] = result{sec: sec, err: err}
		}()
	}
	wg.Wait()

	snap := &Snapshot{}
	snap.Meta = buildMeta()

	var errs []error
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		if err := applySection(snap, r.sec); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return snap, fmt.Errorf("collector errors: %v", errs)
	}

	return snap, nil
}

// ApplySection merges a collected Section into an existing Snapshot.
// Use this to incorporate interactively-collected sections after the main
// concurrent collection completes.
func ApplySection(snap *Snapshot, sec collectors.Section) error {
	return applySection(snap, sec)
}

// New returns an empty Snapshot with metadata populated.
// Use when running collectors individually without snapshot.Create.
func New() *Snapshot {
	s := &Snapshot{}
	s.Meta = buildMeta()
	return s
}

// applySection writes a collected Section into the appropriate Sections field.
func applySection(snap *Snapshot, sec collectors.Section) error {
	switch sec.Name {
	case "env":
		if v, ok := sec.Data.(map[string]string); ok {
			snap.Sections.Env = v
		}
	case "system":
		if v, ok := sec.Data.(*types.SystemInfo); ok {
			snap.Sections.System = v
		}
	case "go":
		if v, ok := sec.Data.(*types.GoInfo); ok {
			snap.Sections.Go = v
		}
	case "packages":
		switch v := sec.Data.(type) {
		case []types.Package:
			snap.Sections.Packages = v
		case types.DNFData:
			snap.Sections.Packages = v.Packages
			snap.Sections.Repositories = v.Repositories
		}
	case "repositories":
		if v, ok := sec.Data.([]types.Repository); ok {
			snap.Sections.Repositories = v
		}
	case "kernel_modules":
		if v, ok := sec.Data.([]types.KernelModule); ok {
			snap.Sections.KernelMods = v
		}
	default:
		return fmt.Errorf("unknown section %q", sec.Name)
	}
	return nil
}

func buildMeta() Meta {
	hostname, _ := os.Hostname()
	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
	}
	return Meta{
		SchemaVersion: SchemaVersion,
		CreatedAt:     time.Now().UTC(),
		Hostname:      hostname,
		User:          username,
		ToolVersion:   version.VersionNumber,
	}
}

func filter(cols []collectors.Collector, opts Options) []collectors.Collector {
	excluded := toSet(opts.Exclude)
	included := toSet(opts.Include)

	var out []collectors.Collector
	for _, c := range cols {
		name := c.Name()
		if excluded[name] {
			continue
		}
		if len(included) > 0 && !included[name] {
			continue
		}
		out = append(out, c)
	}
	return out
}

func toSet(slice []string) map[string]bool {
	m := make(map[string]bool, len(slice))
	for _, s := range slice {
		m[s] = true
	}
	return m
}
