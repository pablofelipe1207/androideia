package main

import (
	"github.com/pablofelipe1207/androideia/cmd"
	"github.com/pablofelipe1207/androideia/internal/version"
)

var (
	v         = "dev"
	buildTime = "unknown"
	commit    = "unknown"
)

func main() {
	// Override defaults from version package with build-time ldflags
	version.Version = v
	version.GitCommit = commit
	version.BuildDate = buildTime
	cmd.Execute()
}
