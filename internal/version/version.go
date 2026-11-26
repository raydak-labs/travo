// Package version holds build stamps injected with -ldflags.
package version

// Version and BuildTime are set at build time; BuildTime is the clock-plausibility floor.
var (
	Version   = "dev"
	BuildTime = ""
)
