package main

import (
	"fmt"
	"os"

	_ "github.com/undrift/drift/internal/preinit" // must be first — see package doc
	"github.com/undrift/drift/internal/cmd"
)

// Version is set via ldflags at build time
var version = "dev"

func main() {
	// Unset the CI env var that preinit set to prevent bubbletea's init()
	// from blocking on a terminal query. Safe to unset now — all init()
	// functions have completed.
	os.Unsetenv("CI")

	cmd.SetVersion(version)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
