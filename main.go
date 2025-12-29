/*
Copyright © 2024 prox contributors
*/
package main

import (
	"github.com/scotty-c/prox/cmd"
	_ "github.com/scotty-c/prox/cmd/config"
	_ "github.com/scotty-c/prox/cmd/ct"
	_ "github.com/scotty-c/prox/cmd/vm"
)

// Build-time variables injected via ldflags
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
	goVersion = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, buildDate, goVersion)
	cmd.Execute()
}
