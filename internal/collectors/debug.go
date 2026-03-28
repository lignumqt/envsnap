package collectors

import (
	"context"
	"fmt"
	"os"
)

type contextKey int

const (
	debugKey          contextKey = iota
	nonInteractiveKey contextKey = iota
)

// WithDebug returns a context with debug mode enabled.
// All collectors that receive this context will emit extra diagnostic output to stderr.
func WithDebug(ctx context.Context) context.Context {
	return context.WithValue(ctx, debugKey, true)
}

// IsDebug reports whether debug mode is enabled in the context.
func IsDebug(ctx context.Context) bool {
	v, _ := ctx.Value(debugKey).(bool)
	return v
}

// WithNonInteractive marks the context so interactive collectors (e.g. kernel
// module TUI selector) fall back to a non-interactive mode automatically.
func WithNonInteractive(ctx context.Context) context.Context {
	return context.WithValue(ctx, nonInteractiveKey, true)
}

// IsNonInteractive reports whether interactive TUI should be suppressed.
func IsNonInteractive(ctx context.Context) bool {
	v, _ := ctx.Value(nonInteractiveKey).(bool)
	return v
}

// debugf prints a formatted debug message to stderr when debug mode is active.
func debugf(ctx context.Context, format string, args ...any) {
	if IsDebug(ctx) {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}
