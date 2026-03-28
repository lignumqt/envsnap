package collectors

import (
	"context"
	"os/exec"
	"strings"

	"github.com/lignumqt/envsnap/internal/types"
)

const GoCollectorName = "go"

// GoCollector captures the installed Go toolchain information.
type GoCollector struct{}

func NewGoCollector() *GoCollector { return &GoCollector{} }

func (c *GoCollector) Name() string { return GoCollectorName }

func (c *GoCollector) Collect(ctx context.Context) (Section, error) {
	info := &types.GoInfo{
		Env: make(map[string]string),
	}

	// go version
	if out, err := exec.CommandContext(ctx, "go", "version").Output(); err == nil {
		// output: "go version go1.26.1 linux/amd64"
		parts := strings.Fields(string(out))
		if len(parts) >= 3 {
			info.Version = parts[2]
		}
	}

	// go env — each line is KEY=VALUE (no quotes on Unix)
	if out, err := exec.CommandContext(ctx, "go", "env").Output(); err == nil {
		scanner := strings.NewReader(string(out))
		buf := make([]byte, 0, len(out))
		buf = append(buf, out...)
		lines := strings.Split(string(buf), "\n")
		_ = scanner
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			idx := strings.IndexByte(line, '=')
			if idx < 0 {
				continue
			}
			key := line[:idx]
			val := strings.Trim(line[idx+1:], `"'`)
			info.Env[key] = val
		}
		// Promote key vars to top-level fields for convenient access.
		if v, ok := info.Env["GOPATH"]; ok && v != "" {
			// already stored in Env
			_ = v
		}
	}

	return Section{Name: GoCollectorName, Data: info}, nil
}
