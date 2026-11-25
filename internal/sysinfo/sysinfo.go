// Package sysinfo reports how much of the machine this process is using:
// memory, CPU time and the free space on the drive that holds the library.
// Each OS-specific part lives in a _unix or _windows file.
package sysinfo

import (
	"runtime/metrics"
	"time"
)

// Memory returns the bytes the Go runtime holds from the OS, minus heap pages
// it has already handed back. It tracks the process's resident size closely
// without any per-OS code (cgo-free SQLite means almost nothing lives outside
// the Go heap).
func Memory() (total, heap uint64) {
	s := []metrics.Sample{
		{Name: "/memory/classes/total:bytes"},
		{Name: "/memory/classes/heap/released:bytes"},
		{Name: "/memory/classes/heap/objects:bytes"},
	}
	metrics.Read(s)
	v := func(i int) uint64 {
		if s[i].Value.Kind() != metrics.KindUint64 {
			return 0
		}
		return s[i].Value.Uint64()
	}
	total, released := v(0), v(1)
	if released < total {
		total -= released
	}
	return total, v(2)
}

// CPUTime is the user plus system CPU time this process has used since it
// started, summed over every core.
func CPUTime() (time.Duration, error) { return cpuTime() }

// Disk reports the free space (available to this user) and the total size of
// the filesystem that holds path.
func Disk(path string) (free, total uint64, err error) { return disk(path) }
