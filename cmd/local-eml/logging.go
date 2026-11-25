package main

import (
	"io"
	"log/slog"
	"os"

	"github.com/kardianos/service"

	"github.com/hwhang0917/local-eml/internal/logrotate"
	"github.com/hwhang0917/local-eml/internal/paths"
)

const logRetentionDays = 30

func setupLogger(p paths.Paths) (*slog.Logger, func()) {
	interactive := service.Interactive()

	level := slog.LevelInfo
	switch os.Getenv("LOCAL_EML_LOG_LEVEL") {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	// Logs carry subjects and addresses, so they get the same owner-only mode
	// as the rest of the data folder. The file rotates daily and rotated
	// days older than logRetentionDays are deleted.
	f, err := logrotate.Open(p.Logs, paths.LogName, logrotate.Options{
		MaxAgeDays: logRetentionDays,
		Perm:       0o600,
	})

	var w io.Writer = os.Stderr
	cleanup := func() {}
	if err == nil {
		w = io.MultiWriter(os.Stderr, f)
		cleanup = func() { _ = f.Close() }
	}

	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if interactive {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}

	return slog.New(h).With(
		slog.String("version", version),
		slog.Int("pid", os.Getpid()),
	), cleanup
}
