package main

import (
	"math"
	"runtime/debug"
)

// DaemonMemLimit is the daemon's Go soft memory limit. Steady RSS is ~19 MB (~12 MB anon) on
// the AXT1800, inside a ~80 MB budget it shares with AdGuard Home (device-facts §11). The
// limit only makes the GC work harder near 32 MiB, so a burst (package list, firmware
// upload) cannot push the router into the OOM killer; it never fails an allocation, and the
// Go runtime caps the GC's CPU share when the live heap exceeds it.
const DaemonMemLimit = 32 << 20

// memoryLimit picks the limit to set: an explicit GOMEMLIMIT is the operator's override and
// already applied by the runtime (ok false); otherwise the daemon default.
func memoryLimit(env string) (limit int64, ok bool) {
	if env != "" {
		return math.MaxInt64, false
	}
	return DaemonMemLimit, true
}

// applyMemoryLimit is called once at daemon start, before any work.
func applyMemoryLimit(getenv func(string) string) {
	if limit, ok := memoryLimit(getenv("GOMEMLIMIT")); ok {
		debug.SetMemoryLimit(limit)
	}
}
