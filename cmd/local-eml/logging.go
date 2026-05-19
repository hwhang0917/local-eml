package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kardianos/service"

	"github.com/hwhang0917/local-eml/internal/paths"
)

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

	logPath := filepath.Join(p.Logs, "local-eml.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)

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
