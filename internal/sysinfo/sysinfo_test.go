package sysinfo

import "testing"

func TestReadings(t *testing.T) {
	if total, heap := Memory(); total == 0 || heap == 0 || heap > total {
		t.Errorf("Memory() = %d, %d", total, heap)
	}
	if _, err := CPUTime(); err != nil {
		t.Errorf("CPUTime: %v", err)
	}
	free, total, err := Disk(t.TempDir())
	if err != nil || total == 0 || free > total {
		t.Errorf("Disk = %d, %d, %v", free, total, err)
	}
}
