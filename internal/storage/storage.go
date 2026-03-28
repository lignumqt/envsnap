package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lignumqt/envsnap/internal/snapshot"
)

const defaultDirName = ".envsnapshot"

// DefaultDir returns the default snapshot storage directory (~/.envsnapshot).
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, defaultDirName), nil
}

// DefaultPath generates a default file path for a new snapshot.
func DefaultPath() (string, error) {
	dir, err := DefaultDir()
	if err != nil {
		return "", err
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	safe := ""
	for _, ch := range ts {
		if ch == ':' {
			safe += "-"
		} else {
			safe += string(ch)
		}
	}
	return filepath.Join(dir, "snapshot-"+safe+".json"), nil
}

// Save serialises snap as pretty-printed JSON to path, creating parent
// directories if necessary.
func Save(path string, snap *snapshot.Snapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("cannot create snapshot directory: %w", err)
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot serialise snapshot: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("cannot write snapshot file: %w", err)
	}
	return nil
}

// Load reads and deserialises a snapshot from path.
func Load(path string) (*snapshot.Snapshot, error) {
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("cannot read snapshot file %q: %w", path, err)
	}

	var snap snapshot.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("cannot parse snapshot file %q: %w", path, err)
	}
	return &snap, nil
}

// List returns all snapshot file paths found in the default directory.
func List() ([]string, error) {
	dir, err := DefaultDir()
	if err != nil {
		return nil, err
	}

	entries, err := filepath.Glob(filepath.Join(dir, "snapshot-*.json"))
	if err != nil {
		return nil, fmt.Errorf("cannot list snapshots in %q: %w", dir, err)
	}
	return entries, nil
}
