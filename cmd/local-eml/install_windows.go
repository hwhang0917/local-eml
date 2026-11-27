//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/kardianos/service"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// On Windows the background server is not a Windows service. A service runs
// as SYSTEM, the most privileged account, and Local Eml installs into the
// user's own folder, which any of the user's programs can write to: whoever
// replaced the file there would run as SYSTEM at the next start. SYSTEM also
// has a home folder of its own, so such a service would not even serve the
// user's library.
//
// Instead Windows starts it when the user signs in, as that user, from the
// per-user Run key. No administrator rights are involved at any point.
const (
	runKeyPath  = `Software\Microsoft\Windows\CurrentVersion\Run`
	runKeyValue = "Local Eml"
)

// runKeyCommand is what Windows runs at sign-in.
func runKeyCommand(exe string, port int) string {
	return `"` + exe + `" ` + strings.Join(backgroundArgs(port), " ")
}

func platformInstall(port int) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	warnAboutLegacyService()

	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open the sign-in startup list: %w", err)
	}
	defer key.Close()
	if err := key.SetStringValue(runKeyValue, runKeyCommand(exe, port)); err != nil {
		return fmt.Errorf("add to the sign-in startup list: %w", err)
	}

	// Start it now too, so installing doesn't need a sign-out.
	stopBackground()
	cmd := exec.Command(exe, backgroundArgs(port)...)
	cmd.Env = append(os.Environ(), detachedEnv+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("registered, but could not start it now: %w", err)
	}
	return cmd.Process.Release()
}

func platformUninstall() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err == nil {
		defer key.Close()
		if err := key.DeleteValue(runKeyValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return fmt.Errorf("remove from the sign-in startup list: %w", err)
		}
	}
	stopBackground()
	warnAboutLegacyService()
	return nil
}

// stopBackground ends the background server this user started, if one runs.
func stopBackground() {
	pid, ok := readBackgroundPID()
	if !ok {
		return
	}
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	// The pid file can outlive its process and the number be reused, so only
	// end a process that is this same program.
	if !sameExecutable(h) {
		return
	}
	_ = windows.TerminateProcess(h, 0)
	_, _ = windows.WaitForSingleObject(h, 5000)
}

func sameExecutable(h windows.Handle) bool {
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return false
	}
	exe, err := os.Executable()
	return err == nil && strings.EqualFold(windows.UTF16ToString(buf[:n]), exe)
}

// warnAboutLegacyService removes the Windows service that versions before
// 0.18.0 registered, or says how to when that needs an administrator.
func warnAboutLegacyService() {
	s, err := newService(defaultPort)
	if err != nil {
		return
	}
	if _, err := s.Status(); errors.Is(err, service.ErrNotInstalled) {
		return
	}
	_ = s.Stop()
	if err := s.Uninstall(); err == nil {
		fmt.Println("Removed the Windows service an older version registered.")
		return
	}
	fmt.Println()
	fmt.Println("An older version registered a Windows service named \"local-eml\" that runs as SYSTEM.")
	fmt.Println("It is no longer used and should be removed. In PowerShell opened as administrator, run:")
	fmt.Println("  sc.exe stop local-eml; sc.exe delete local-eml")
	fmt.Println()
}
