//go:build !windows

package main

import "errors"

// Elsewhere the service manager (systemd, launchd) does this work; see
// install.go.
func platformInstall(int) (int, error) { return 0, errors.New("not used on this platform") }
func platformUninstall() error         { return errors.New("not used on this platform") }
