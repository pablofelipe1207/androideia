package main

import (
	"github.com/pablofelipe1207/androideia/cmd"
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
