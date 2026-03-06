package version

import (
	"fmt"
	"runtime"
)

// Set at build time via ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Short returns a concise version string (e.g. "v0.1.0").
func Short() string {
	return Version
}

// Info returns a multi-line version string with all build metadata.
func Info() string {
	return fmt.Sprintf("dagryn %s\ncommit: %s\nbuilt:  %s\ngo:     %s\nos/arch: %s/%s",
		Version, Commit, BuildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
