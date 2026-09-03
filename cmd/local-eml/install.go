package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/kardianos/service"
)

type stubProgram struct{}

func (stubProgram) Start(service.Service) error { return nil }
func (stubProgram) Stop(service.Service) error  { return nil }

// systemdUnit overrides kardianos's default systemd template. The default
// hardcodes RestartSec=120, which leaves the service down for two full
// minutes after any clean exit — including the binary swap performed by the
// in-app update flow. RestartSec=2 here makes the auto-restart-on-exit path
// fast, so even if our explicit `systemctl --user restart` nudge doesn't
// land (PATH issue, dbus hiccup), systemd brings us back within seconds.
//
// The template otherwise mirrors the kardianos default for systemd-user
// services. It's a `text/template`, executed against kardianos's internal
// context, so the variables ({{.Description}}, {{.Path}}, etc.) match the
// upstream template surface.
const systemdUnit = `[Unit]
Description={{.Description}}
ConditionFileIsExecutable={{.Path|cmdEscape}}

[Service]
ExecStart={{.Path|cmdEscape}}{{range .Arguments}} {{.|cmd}}{{end}}
Restart=always
RestartSec=2

[Install]
WantedBy=default.target
`

// serveArgs is what the service manager runs. The port is baked into the
// registration; changing it later means uninstall + install.
func serveArgs(port int) []string {
	if port == defaultPort {
		return []string{"serve"}
	}
	return []string{"serve", "--port", strconv.Itoa(port)}
}

func newService(port int) (service.Service, error) {
	cfg := &service.Config{
		Name:        "local-eml",
		DisplayName: "Local Eml",
		Description: "Local EML viewer (loopback-only HTTP server)",
		Arguments:   serveArgs(port),
		Option: service.KeyValue{
			"UserService":   true,
			"RunAtLoad":     true,
			"KeepAlive":     true,
			"SystemdScript": systemdUnit,
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

func parseInstallFlags(args []string) (yes bool, port int) {
	fs := flag.NewFlagSet("install-flags", flag.ExitOnError)
	yFlag := fs.Bool("y", false, "skip the confirmation prompt")
	yesFlag := fs.Bool("yes", false, "skip the confirmation prompt")
	portFlag := fs.Int("port", defaultPort, "TCP port the service listens on (loopback only)")
	_ = fs.Parse(args)
	if *portFlag < 1 || *portFlag > 65535 {
		fmt.Fprintf(os.Stderr, "invalid --port %d: must be 1-65535\n", *portFlag)
		os.Exit(2)
	}
	return *yFlag || *yesFlag, *portFlag
}

func confirm(prompt string) bool {
	fmt.Printf("%s [Y/n] ", prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
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
	yes, port := parseInstallFlags(args)
	s, err := newService(port)
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
	fmt.Println("  Command:      local-eml", strings.Join(serveArgs(port), " "))
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
	fmt.Printf("Installed and started. Open http://localhost:%d\n", port)
	return 0
}

func runUninstall(args []string) int {
	yes, _ := parseInstallFlags(args)
	// Only the service name matters for uninstall; the port is irrelevant.
	s, err := newService(defaultPort)
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
