package version

// These variables are injected at build time via -ldflags.
var (
	VersionNumber = "dev"
	GitCommit     = "none"
	BuildDate     = "unknown"
)

// String returns a human-readable version string.
func String() string {
	return VersionNumber + " (" + GitCommit + ") built " + BuildDate
}
