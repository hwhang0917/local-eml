package server

import "testing"

func TestPlanRestart_LinuxAlwaysIncludesNoBlock(t *testing.T) {
	// Regression guard: dropping --no-block re-introduces the original bug.
	// Without it, `systemctl --user restart` waits for the stop+start sequence
	// to complete, which can never happen because the systemctl process lives
	// in the very cgroup systemd is about to kill — and even if it survived,
	// the synchronous restart deadlocks (it's waiting on its own caller to
	// die). --no-block enqueues the job and returns immediately.
	p := planRestart("linux")
	if !p.available {
		t.Fatal("planRestart(linux): expected available=true")
	}
	if p.cmd != "systemctl" {
		t.Errorf("cmd = %q, want systemctl", p.cmd)
	}
	if !contains(p.args, "--no-block") {
		t.Errorf("args=%v missing --no-block (would deadlock)", p.args)
	}
	if !contains(p.args, "--user") {
		t.Errorf("args=%v missing --user (we install a user service)", p.args)
	}
	if !contains(p.args, "restart") {
		t.Errorf("args=%v missing 'restart' verb", p.args)
	}
}

func TestPlanRestart_DarwinNoOp(t *testing.T) {
	// launchd KeepAlive auto-restart fires fast; nudging would be redundant.
	p := planRestart("darwin")
	if p.available {
		t.Error("darwin: expected available=false (launchd handles it)")
	}
}

func TestPlanRestart_UnknownNoOp(t *testing.T) {
	p := planRestart("plan9")
	if p.available {
		t.Error("plan9: expected available=false")
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
