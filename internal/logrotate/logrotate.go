// Package logrotate writes the app log to one active file and rotates it
// once a day, deleting rotated files past a retention window.
//
// The active file keeps its fixed name so `tail -F` and the docs keep
// working. When the local date changes, the active file is renamed to
// <base>-YYYY-MM-DD<ext> (the date it covers) and a fresh one is opened.
// The day a file covers is taken from its modification time, so a process
// that starts on a new day rotates yesterday's file first: frequent
// restarts never lose the boundary, and lines are only ever appended.
package logrotate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const dateLayout = "2006-01-02"

// Writer is an io.Writer safe for concurrent use.
type Writer struct {
	dir     string
	name    string // active file name, e.g. local-eml.log
	maxAge  int    // days of rotated files to keep
	perm    os.FileMode
	now     func() time.Time
	onError func(error)

	mu     sync.Mutex
	f      *os.File
	day    string // local date the active file covers
	closed bool
}

// Options tunes a Writer. Zero values pick the defaults.
type Options struct {
	MaxAgeDays int              // default 30
	Perm       os.FileMode      // default 0o600
	Now        func() time.Time // for tests
	OnError    func(error)      // rotation/pruning failures; default stderr
}

// Open opens (or creates) dir/name for appending, rotating it first if it
// was last written on an earlier day, and prunes expired rotated files.
func Open(dir, name string, o Options) (*Writer, error) {
	w := &Writer{dir: dir, name: name, maxAge: o.MaxAgeDays, perm: o.Perm, now: o.Now, onError: o.OnError}
	if w.maxAge <= 0 {
		w.maxAge = 30
	}
	if w.perm == 0 {
		w.perm = 0o600
	}
	if w.now == nil {
		w.now = time.Now
	}
	if w.onError == nil {
		w.onError = func(err error) { fmt.Fprintln(os.Stderr, "logrotate:", err) }
	}

	today := w.now().Format(dateLayout)
	if fi, err := os.Stat(w.activePath()); err == nil && fi.Size() > 0 {
		if day := fi.ModTime().Local().Format(dateLayout); day < today {
			if err := w.archive(day); err != nil {
				w.onError(err)
			}
		}
	}
	if err := w.openActive(today); err != nil {
		return nil, err
	}
	w.prune()
	return w, nil
}

func (w *Writer) activePath() string { return filepath.Join(w.dir, w.name) }

func (w *Writer) openActive(day string) error {
	f, err := os.OpenFile(w.activePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, w.perm)
	if err != nil {
		return err
	}
	w.f, w.day = f, day
	return nil
}

// archive renames the (closed) active file to its dated name. A second
// rotation for the same day, e.g. after the clock was set back, gets a
// numeric suffix instead of overwriting.
func (w *Writer) archive(day string) error {
	ext := filepath.Ext(w.name)
	base := strings.TrimSuffix(w.name, ext)
	dst := filepath.Join(w.dir, base+"-"+day+ext)
	for i := 1; ; i++ {
		if _, err := os.Lstat(dst); os.IsNotExist(err) {
			break
		}
		dst = filepath.Join(w.dir, fmt.Sprintf("%s-%s.%d%s", base, day, i, ext))
	}
	return os.Rename(w.activePath(), dst)
}

// Write appends p to the active file, rotating first when the day changed.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, os.ErrClosed
	}
	today := w.now().Format(dateLayout)
	if w.f != nil && today != w.day {
		w.rotate(today)
	}
	if w.f == nil {
		// A reopen failed earlier; try again rather than going silent.
		if err := w.openActive(today); err != nil {
			return 0, err
		}
	}
	return w.f.Write(p)
}

// rotate never leaves the writer without a file: if the rename fails (say
// another program holds the file open on Windows), it keeps appending to
// the same file and tries again the next day.
func (w *Writer) rotate(today string) {
	prev := w.day
	if err := w.f.Close(); err != nil {
		w.onError(err)
	}
	if err := w.archive(prev); err != nil {
		w.onError(err)
	}
	if err := w.openActive(today); err != nil {
		w.onError(err)
		w.f = nil
		return
	}
	w.prune()
}

// prune deletes rotated files whose date is more than maxAge days ago.
// Only names this package produces are touched.
func (w *Writer) prune() {
	ext := filepath.Ext(w.name)
	prefix := strings.TrimSuffix(w.name, ext) + "-"
	cutoff := w.now().AddDate(0, 0, -w.maxAge).Format(dateLayout)
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		w.onError(err)
		return
	}
	for _, e := range entries {
		day, ok := RotatedDate(e.Name(), prefix, ext)
		if !ok || day >= cutoff {
			continue
		}
		if err := os.Remove(filepath.Join(w.dir, e.Name())); err != nil {
			w.onError(err)
		}
	}
}

// RotatedDate reports the YYYY-MM-DD date in a rotated file name such as
// local-eml-2026-11-24.log or local-eml-2026-11-24.1.log.
func RotatedDate(name, prefix, ext string) (string, bool) {
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ext) {
		return "", false
	}
	rest := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ext)
	if len(rest) < len(dateLayout) {
		return "", false
	}
	day := rest[:len(dateLayout)]
	if _, err := time.Parse(dateLayout, day); err != nil {
		return "", false
	}
	if tail := rest[len(dateLayout):]; tail != "" {
		if tail[0] != '.' || strings.Trim(tail[1:], "0123456789") != "" || len(tail) == 1 {
			return "", false
		}
	}
	return day, true
}

// Close closes the active file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}
