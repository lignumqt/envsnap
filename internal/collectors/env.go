package collectors

import (
	"context"
	"os"
	"strings"
)

const EnvCollectorName = "env"

// EnvCollector captures the current process environment variables.
type EnvCollector struct{}

func NewEnvCollector() *EnvCollector { return &EnvCollector{} }

func (c *EnvCollector) Name() string { return EnvCollectorName }

func (c *EnvCollector) Collect(_ context.Context) (Section, error) {
	raw := os.Environ()
	result := make(map[string]string, len(raw))
	for _, kv := range raw {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			result[kv] = ""
			continue
		}
		result[kv[:idx]] = kv[idx+1:]
	}
	return Section{Name: EnvCollectorName, Data: result}, nil
}
