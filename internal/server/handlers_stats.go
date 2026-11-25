package server

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/hwhang0917/local-eml/internal/sysinfo"
)

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.Store.Stats(r.Context(), tzOffset(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// tzOffset reads ?tz_offset, the browser's offset east of UTC in minutes
// (-new Date().getTimezoneOffset()). Missing or junk means UTC.
func tzOffset(r *http.Request) int {
	n, err := strconv.Atoi(r.URL.Query().Get("tz_offset"))
	if err != nil {
		return 0
	}
	return n
}

// diskCacheTTL keeps the stats page's few-second refresh from walking every
// .eml file each time; a library only changes size on import or delete.
const diskCacheTTL = time.Minute

type appDisk struct {
	DBBytes   int64  `json:"db_bytes"`
	EMLBytes  int64  `json:"eml_bytes"`
	EMLFiles  int    `json:"eml_files"`
	LogsBytes int64  `json:"logs_bytes"`
	FreeBytes uint64 `json:"free_bytes"`
	// TotalBytes is the size of the whole drive the library is on.
	TotalBytes uint64 `json:"total_bytes"`
	MeasuredAt int64  `json:"measured_at"`
}

type appStats struct {
	MemoryBytes uint64 `json:"memory_bytes"`
	HeapBytes   uint64 `json:"heap_bytes"`
	// CPUPercent is this process's share of the whole machine (all cores)
	// since the previous call, or since start on the first one.
	CPUPercent float64  `json:"cpu_percent"`
	NumCPU     int      `json:"num_cpu"`
	Goroutines int      `json:"goroutines"`
	UptimeSec  int64    `json:"uptime_sec"`
	GoVersion  string   `json:"go_version"`
	Version    string   `json:"version"`
	Disk       *appDisk `json:"disk"`
}

var (
	processStart = time.Now()

	appMu      sync.Mutex
	lastCPU    time.Duration
	lastCPUAt  = processStart
	cachedDisk *appDisk
	diskRoot   string
)

func (s *Server) handleAppStats(w http.ResponseWriter, r *http.Request) {
	mem, heap := sysinfo.Memory()
	out := appStats{
		MemoryBytes: mem,
		HeapBytes:   heap,
		NumCPU:      runtime.NumCPU(),
		Goroutines:  runtime.NumGoroutine(),
		UptimeSec:   int64(time.Since(processStart).Seconds()),
		GoVersion:   runtime.Version(),
		Version:     s.Version,
	}

	appMu.Lock()
	defer appMu.Unlock()
	if cpu, err := sysinfo.CPUTime(); err == nil {
		now := time.Now()
		if wall := now.Sub(lastCPUAt); wall > 0 && cpu >= lastCPU {
			out.CPUPercent = float64(cpu-lastCPU) / float64(wall) / float64(out.NumCPU) * 100
		}
		lastCPU, lastCPUAt = cpu, now
	}

	if s.Importer != nil {
		p := s.Importer.Paths
		if cachedDisk == nil || diskRoot != p.Base ||
			time.Since(time.Unix(cachedDisk.MeasuredAt, 0)) > diskCacheTTL {
			d := &appDisk{MeasuredAt: time.Now().Unix()}
			// The WAL and shared-memory files sit beside the database and can
			// hold a lot right after a big import.
			for _, f := range []string{p.DBFile(), p.DBFile() + "-wal", p.DBFile() + "-shm"} {
				if fi, err := os.Stat(f); err == nil {
					d.DBBytes += fi.Size()
				}
			}
			d.EMLBytes, d.EMLFiles = dirSize(p.EML)
			d.LogsBytes, _ = dirSize(p.Logs)
			d.FreeBytes, d.TotalBytes, _ = sysinfo.Disk(p.Base)
			cachedDisk, diskRoot = d, p.Base
		}
		out.Disk = cachedDisk
	}
	writeJSON(w, http.StatusOK, out)
}

// dirSize sums the regular files under dir. Unreadable entries are skipped:
// a rough number beats an error on a stats page.
func dirSize(dir string) (bytes int64, files int) {
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			bytes += fi.Size()
			files++
		}
		return nil
	})
	return bytes, files
}
