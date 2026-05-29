package server

import (
	"fmt"
	"os/exec"
	"runtime"
)

// restartPlan describes how to ask the platform's service manager to bring
// us back up out-of-band, instead of relying on the auto-restart-after-exit
// path. The latter is rate-limited on systemd (RestartSec=120 in the
// kardianos default unit), so a user-visible update would otherwise sit
// down for 2 minutes after the binary swap.
type restartPlan struct {
	available bool
	cmd       string
	args      []string
	// why explains what this command achieves; surfaces in logs so it's
	// obvious why we shelled out.
	why string
}

// planRestart returns the platform-appropriate "ask the service manager to
// restart me" command, if one applies. The systemctl invocation passes
// --no-block on purpose: an explicit restart job does NOT honor RestartSec
// (that only gates auto-restart-after-failure), and --no-block also avoids
// systemctl waiting for the full stop+start sequence while it itself lives
// in our soon-to-be-killed cgroup.
func planRestart(goos string) restartPlan {
	switch goos {
	case "linux":
		return restartPlan{
			available: true,
			cmd:       "systemctl",
			args:      []string{"--user", "--no-block", "restart", "local-eml"},
			why:       "queue an explicit systemd restart so RestartSec is bypassed",
		}
	case "darwin":
		// launchd KeepAlive auto-restart fires within ~1 second of the
		// process exiting; no nudge needed.
		return restartPlan{available: false, why: "launchd KeepAlive handles immediate auto-restart on exit"}
	case "windows":
		// Windows SCM auto-restart depends on FailureActions, which the
		// kardianos default doesn't configure. We rely on auto-restart if
		// the user has configured it; otherwise the update is one-shot.
		return restartPlan{available: false, why: "rely on Windows SCM FailureActions if configured"}
	default:
		return restartPlan{available: false, why: "no known service manager for " + goos}
	}
}

// requestImmediateRestart fires the planned restart command, fire-and-forget.
// Returns nil if there's no platform-applicable action OR the command was
// successfully queued; non-nil if we tried and the spawn itself failed
// (rare — exec/lookpath errors).
func requestImmediateRestart() error {
	p := planRestart(runtime.GOOS)
	if !p.available {
		return nil
	}
	if _, err := exec.LookPath(p.cmd); err != nil {
		return fmt.Errorf("%s not in PATH: %w", p.cmd, err)
	}
	cmd := exec.Command(p.cmd, p.args...)
	// Run blocks until the command exits, but with --no-block on systemctl
	// the command returns as soon as the dbus message is enqueued — well
	// before systemd actually SIGTERMs us.
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", p.cmd, p.args, err)
	}
	return nil
}
