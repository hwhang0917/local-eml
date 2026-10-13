//go:build windows

package tempsession

import (
	"errors"
	"syscall"
)

// stillActive is STILL_ACTIVE, the exit code Windows reports for a process
// that hasn't exited. An exited process can stay openable while anyone holds
// a handle to it, so a successful open alone doesn't prove it's running.
const stillActive = 259

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		// Access denied means it exists under another account.
		return errors.Is(err, syscall.ERROR_ACCESS_DENIED)
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return true
	}
	return code == stillActive
}
