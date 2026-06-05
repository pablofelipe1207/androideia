package main

import (
	"github.com/mobiai/androideai-core/cmd"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	cmd.Version = version
	cmd.BuildTime = buildTime
	cmd.Execute()
}
