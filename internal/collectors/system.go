package collectors

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/lignumqt/envsnap/internal/types"
)

const SystemCollectorName = "system"

type SystemCollector struct{}

func NewSystemCollector() *SystemCollector { return &SystemCollector{} }

func (c *SystemCollector) Name() string { return SystemCollectorName }

func (c *SystemCollector) Collect(ctx context.Context) (Section, error) {
	info := &types.SystemInfo{
		Shell: os.Getenv("SHELL"),
		Arch:  runtime.GOARCH,
	}
	parseOSRelease(info)
	if out, err := exec.CommandContext(ctx, "uname", "-r").Output(); err == nil {
		info.KernelVersion = strings.TrimSpace(string(out))
	}
	return Section{Name: SystemCollectorName, Data: info}, nil
}

func parseOSRelease(info *types.SystemInfo) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return
	}
	defer f.Close()
	fields := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := line[:idx]
		val := strings.Trim(line[idx+1:], `"`)
		fields[key] = val
	}
	info.OS = fields["NAME"]
	info.OSVersion = fields["VERSION_ID"]
	if info.OS == "" {
		info.OS = fields["ID"]
	}
}
