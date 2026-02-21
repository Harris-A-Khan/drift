// Package preinit runs before all other package init() functions.
//
// It must be imported with a blank identifier in main.go BEFORE any other
// internal imports so that its init() executes first in Go's dependency-
// ordered initialization.
//
// Purpose: bubbletea v1.x has an init() that calls lipgloss.HasDarkBackground(),
// which sends an ANSI OSC query to the terminal and blocks waiting for a
// response. Terminals that don't respond to OSC 11 (background color query)
// cause the entire program to hang — even for commands like "drift --help".
//
// Setting CI=1 causes termenv to treat stdout as a non-TTY, skipping the
// query entirely. We unset it in main() so it doesn't affect runtime behavior.
package preinit

import "os"

func init() {
	os.Setenv("CI", "1")
}
