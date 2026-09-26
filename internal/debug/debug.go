// Package debug gates diagnostic logging on the global --debug flag or the
// CUBEHAUL_DEBUG environment variable.
package debug

import (
	"fmt"
	"os"
	"sync/atomic"
)

// enabled mirrors the --debug flag. It is set once, before any command runs, so
// the atomic is only there to keep the race detector honest about goroutines
// started later (retries, concurrent requests).
var enabled atomic.Bool

// Set records the --debug flag.
func Set(on bool) { enabled.Store(on) }

// Enabled reports whether diagnostics are on: either --debug was passed or
// CUBEHAUL_DEBUG is set to a non-empty value.
func Enabled() bool {
	return enabled.Load() || os.Getenv("CUBEHAUL_DEBUG") != ""
}

// Printf writes one diagnostic line to stderr when Enabled. Diagnostics never
// touch stdout, which stays reserved for results.
func Printf(format string, args ...any) {
	if Enabled() {
		fmt.Fprintf(os.Stderr, "cubehaul: "+format+"\n", args...)
	}
}
