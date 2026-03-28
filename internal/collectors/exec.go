package collectors

import (
	"context"
	"os/exec"
)

// newExecCmd creates an *exec.Cmd with context.
func newExecCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
