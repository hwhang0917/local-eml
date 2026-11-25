//go:build windows

package sysinfo

import (
	"time"

	"golang.org/x/sys/windows"
)

func cpuTime() (time.Duration, error) {
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(windows.CurrentProcess(), &creation, &exit, &kernel, &user); err != nil {
		return 0, err
	}
	// Filetime durations count 100 ns ticks.
	ticks := func(f windows.Filetime) int64 {
		return int64(f.HighDateTime)<<32 | int64(f.LowDateTime)
	}
	return time.Duration((ticks(kernel) + ticks(user)) * 100), nil
}

func disk(path string) (free, total uint64, err error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree); err != nil {
		return 0, 0, err
	}
	return free, total, nil
}
