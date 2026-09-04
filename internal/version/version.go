// Package version holds the single cubehaul version string.
//
// The source tree defaults to "dev" and never needs bumping: the release
// workflow rewrites the value at link time from the pushed tag, making the
// tag itself the source of truth:
//
//	go build -ldflags "-X cubehaul/internal/version.value=v0.1.0" .
//
// Everything user-visible (cubehaul --version and the default User-Agent)
// reads from here, so a release cannot leave stale copies behind.
package version

// value is rewritten at link time via -ldflags -X. Unexported on purpose:
// readers go through Value.
var value = "dev"

// Value returns the build version.
func Value() string {
	return value
}
