package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kardianos/service"
)

type stubProgram struct{}

func (stubProgram) Start(service.Service) error { return nil }
func (stubProgram) Stop(service.Service) error  { return nil }

func newService() (service.Service, error) {
	cfg := &service.Config{
		Name:        "local-eml",
		DisplayName: "Local Eml",
		Description: "Local EML viewer (loopback-only HTTP server)",
		Arguments:   []string{"serve"},
		Option: service.KeyValue{
			"UserService": true,
			"RunAtLoad":   true,
			"KeepAlive":   true,
		},
	}
	return service.New(stubProgram{}, cfg)
}

func describePlatform() string {
	switch runtime.GOOS {
	case "linux":
		return "Linux — systemd user unit (no root required)"
	case "darwin":
		return "macOS — launchd LaunchAgent (no root required)"
	case "windows":
		return "Windows — Service Control Manager (admin required)"
	default:
		return runtime.GOOS
	}
}

func describePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch runtime.GOOS {
	case "linux":
		return filepath.Join(home, ".config", "systemd", "user", "local-eml.service")
	case "darwin":
		return filepath.Join(home, "Library", "LaunchAgents", "local-eml.plist")
	case "windows":
		return `Windows Service Control Manager: "local-eml"`
	default:
		return ""
	}
}

func parseYesFlag(args []string) (yes bool, rest []string) {
	fs := flag.NewFlagSet("install-flags", flag.ExitOnError)
	yFlag := fs.Bool("y", false, "skip the confirmation prompt")
	yesFlag := fs.Bool("yes", false, "skip the confirmation prompt")
	_ = fs.Parse(args)
	return *yFlag || *yesFlag, fs.Args()
}

func confirm(prompt string) bool {
	fmt.Printf("%s [Y/n] ", prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, os.ErrClosed) {
			return true
		}
		// Empty stdin (EOF) → take the default (Yes).
		if err.Error() == "EOF" {
			fmt.Println()
			return true
		}
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer == "" {
		return true
	}
	return answer == "y" || answer == "yes"
}

func runInstall(args []string) int {
	yes, _ := parseYesFlag(args)
	s, err := newService()
	if err != nil {
		fmt.Fprintln(os.Stderr, "service config:", err)
		return 1
	}
	exe, _ := os.Executable()

	fmt.Println("Install Local Eml as a background service:")
	fmt.Println("  Platform:    ", describePlatform())
	fmt.Println("  Service name: local-eml")
	fmt.Println("  Binary path: ", exe)
	fmt.Println("  Service file:", describePath())
	fmt.Println("  Command:      local-eml serve")
	fmt.Println("  Auto-start:   yes (RunAtLoad + KeepAlive)")
	fmt.Println()

	if !yes && !confirm("Proceed?") {
		fmt.Println("Aborted.")
		return 1
	}

	if err := s.Install(); err != nil {
		fmt.Fprintln(os.Stderr, "install failed:", err)
		fmt.Fprintln(os.Stderr, "(if the service already exists, run `local-eml uninstall` first)")
		return 1
	}
	if err := s.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "service registered but failed to start:", err)
		return 1
	}
	fmt.Println()
	fmt.Println("Installed and started. Open http://localhost:7878")
	return 0
}

func runUninstall(args []string) int {
	yes, _ := parseYesFlag(args)
	s, err := newService()
	if err != nil {
		fmt.Fprintln(os.Stderr, "service config:", err)
		return 1
	}

	fmt.Println("Uninstall Local Eml background service:")
	fmt.Println("  Platform:    ", describePlatform())
	fmt.Println("  Service file:", describePath())
	fmt.Println()
	fmt.Println("Your data in ~/.local-eml/ (EMLs + database) will NOT be touched.")
	fmt.Println()

	if !yes && !confirm("Proceed?") {
		fmt.Println("Aborted.")
		return 1
	}

	if err := s.Stop(); err != nil {
		// best-effort; service may already be stopped
		fmt.Fprintln(os.Stderr, "stop (continuing):", err)
	}
	if err := s.Uninstall(); err != nil {
		fmt.Fprintln(os.Stderr, "uninstall failed:", err)
		return 1
	}
	fmt.Println()
	fmt.Println("Uninstalled.")
	return 0
}
