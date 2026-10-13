//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

const (
	// detachedEnv marks the relaunched copy so it never relaunches again.
	detachedEnv = "LOCAL_EML_DETACHED"

	// createNoWindow is CREATE_NO_WINDOW. The child still has a console, just
	// never a visible one, so anything it starts inherits the hidden console
	// instead of popping up a window of its own.
	createNoWindow = 0x08000000
)

var procGetConsoleProcessList = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleProcessList")

// ownsConsole reports whether this process is alone on its console, which
// means Windows created the console just for it: a double-click in Explorer
// or a shortcut. Run from cmd or PowerShell, the shell shares the console.
func ownsConsole() bool {
	var pids [2]uint32
	n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	return n == 1
}

// relaunchWithoutConsole restarts the app with its console hidden and reports
// whether it did, in which case the caller exits and its console window
// closes with it.
//
// The binary stays a console program on purpose. Built as a GUI program it
// would never show a console, but then the terminal commands lose their
// output, and PowerShell stops waiting for it, which breaks the installer's
// "Proceed?" prompt.
func relaunchWithoutConsole() bool {
	if os.Getenv(detachedEnv) != "" || !ownsConsole() {
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Env = append(os.Environ(), detachedEnv+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
	if err := cmd.Start(); err != nil {
		// Showing the console is better than not starting at all.
		return false
	}
	_ = cmd.Process.Release()
	return true
}
