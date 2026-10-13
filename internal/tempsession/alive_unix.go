//go:build !windows

package tempsession

import (
	"errors"
	"syscall"
)

// processAlive uses signal 0, which checks for existence without delivering
// anything. EPERM means the process exists but belongs to someone else.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
