//go:build !windows

package main

// relaunchWithoutConsole is Windows-only: elsewhere, launching from a file
// manager doesn't open a terminal window.
func relaunchWithoutConsole() bool { return false }
