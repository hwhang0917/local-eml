//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/kardianos/service"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/hwhang0917/local-eml/internal/paths"
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

// startWait is how long install waits for the background server to report
// the port it is listening on.
const startWait = 15 * time.Second

func platformInstall(port int) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	warnAboutLegacyService()

	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return 0, fmt.Errorf("open the sign-in startup list: %w", err)
	}
	defer key.Close()
	if err := key.SetStringValue(runKeyValue, runKeyCommand(exe, port)); err != nil {
		return 0, fmt.Errorf("add to the sign-in startup list: %w", err)
	}

	// Start it now too, so installing doesn't need a sign-out.
	stopBackground()
	cmd := exec.Command(exe, backgroundArgs(port)...)
	cmd.Env = append(os.Environ(), detachedEnv+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("registered, but could not start it now: %w", err)
	}
	return waitForBackground(cmd)
}

// waitForBackground waits until the server just started says which port it
// listens on. The default port moves to the next free one when another
// program holds it, so the port asked for is not always the port in use.
func waitForBackground(cmd *exec.Cmd) (int, error) {
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	deadline := time.After(startWait)
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-exited:
			return 0, startFailure("it stopped right after starting")
		case <-deadline:
			return 0, startFailure("it did not report a port in time")
		case <-tick.C:
			if pid, port, ok := readBackgroundPID(); ok && pid == cmd.Process.Pid {
				return port, nil
			}
		}
	}
}

// startFailure explains a background server that did not come up. The usual
// reason is another Local Eml already answering on the port, such as one
// running inside WSL, whose ports Windows forwards to itself.
func startFailure(what string) error {
	msg := "registered for sign-in, but the server did not start now: " + what + "."
	if other := findExisting(); other != 0 {
		msg += fmt.Sprintf("\nAnother Local Eml already answers at http://localhost:%d (for example one running in WSL)."+
			"\nStop that one, or choose a different port: local-eml install --port 7900", other)
	}
	if p, err := paths.Resolve(); err == nil {
		msg += "\nThe log is at " + p.LogFile()
	}
	return errors.New(msg)
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
	pid, _, ok := readBackgroundPID()
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
