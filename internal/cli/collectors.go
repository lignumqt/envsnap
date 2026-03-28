package cli

import (
	"github.com/lignumqt/envsnap/internal/collectors"
)

// defaultCollectors returns all built-in collectors in priority order.
// The kernel modules collector is always last because it's interactive.
func defaultCollectors() []collectors.Collector {
	return []collectors.Collector{
		collectors.NewEnvCollector(),
		collectors.NewSystemCollector(),
		collectors.NewGoCollector(),
		collectors.NewPackagesCollector(),
		collectors.NewKernelModsCollector(),
	}
}
