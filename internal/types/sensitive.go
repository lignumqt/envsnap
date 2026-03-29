package types

import "strings"

// sensitiveEnvPrefixes lists key prefixes (and exact names) that are treated as
// sensitive and excluded from diffs, exports and every other output.
// Add entries here to extend the filter globally.
var sensitiveEnvPrefixes = []string{
	"AWS_",
	"GITHUB_TOKEN",
	"SECRET",
	"PASSWORD",
	"TOKEN",
	"KEY",
	"PASSWD",
}

// IsSensitive reports whether an environment variable name likely holds a
// credential, token or other secret and should not appear in any output.
func IsSensitive(key string) bool {
	for _, prefix := range sensitiveEnvPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}
