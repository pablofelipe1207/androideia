package version

import (
	"fmt"
	"runtime"
)

// Version information for androideai-core.
// These are set at build time via -ldflags.
var (
	Version   = "1.0.0"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// Full returns the full version string with build metadata.
func Full() string {
	return fmt.Sprintf("androideai-core %s (commit: %s, built: %s, %s/%s)",
		Version, GitCommit, BuildDate, runtime.GOOS, runtime.GOARCH)
}

// Short returns just the version number.
func Short() string {
	return Version
}

// Banner prints the version banner to stdout.
func Banner() {
	fmt.Printf("androideai-core v%s\n", Version)
	fmt.Printf("  Go: %s | OS: %s | Arch: %s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
