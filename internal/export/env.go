package export

import (
	"fmt"
	"strings"

	"github.com/lignumqt/envsnap/internal/snapshot"
)

// renderEnv produces a shell env-variable fragment that can be sourced directly:
//
//	source <(envsnap export --format env snapshot.json)
//
// Unlike script, dockerfile, and ansible, this format exports ALL non-sensitive
// variables (not restricted to the important-prefix list) for maximum fidelity.
func renderEnv(snap *snapshot.Snapshot, opts Options) (string, error) {
	var sb strings.Builder

	if opts.SkipEnv || len(snap.Sections.Env) == 0 {
		fmt.Fprintf(&sb, "# envsnap: no environment variables in snapshot\n")
		return sb.String(), nil
	}

	// No importantOnly filter — export everything non-sensitive.
	envVars := filteredEnvVars(snap.Sections.Env, false)
	if len(envVars) == 0 {
		fmt.Fprintf(&sb, "# envsnap: all captured variables were filtered as sensitive\n")
		return sb.String(), nil
	}

	fmt.Fprintf(&sb, "# envsnap %s — export --format env\n", snap.Meta.ToolVersion)
	fmt.Fprintf(&sb, "# Source host: %s  user: %s  captured: %s\n",
		snap.Meta.Hostname, snap.Meta.User,
		snap.Meta.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&sb, "# Usage: source <(envsnap export --format env snapshot.json)\n")
	fmt.Fprintf(&sb, "#        eval \"$(envsnap export --format env snapshot.json)\"\n\n")

	for _, k := range sortedKeys(envVars) {
		fmt.Fprintf(&sb, "export %s=%s\n", k, shellQuote(envVars[k]))
	}

	fmt.Fprintf(&sb, "\n# %d variables exported (%d sensitive variables omitted)\n",
		len(envVars), len(snap.Sections.Env)-len(envVars))

	return strings.TrimRight(sb.String(), "\n") + "\n", nil
}
