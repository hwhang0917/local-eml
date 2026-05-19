package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/paths"
	"github.com/hwhang0917/local-eml/internal/server"
	"github.com/hwhang0917/local-eml/internal/store"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		os.Exit(runServe(os.Args[2:]))
	case "version", "-V", "--version":
		fmt.Printf("local-eml v%s\n", version)
	case "-h", "--help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "local-eml v%s\n", version)
	fmt.Fprintln(os.Stderr, "usage: local-eml <serve|version> [flags]")
	fmt.Fprintln(os.Stderr, "  serve [--port 7878]   run the local web server (loopback only)")
	fmt.Fprintln(os.Stderr, "  version | -V | --version")
}

func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 7878, "TCP port to listen on (loopback only)")
	_ = fs.Parse(args)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	p, err := paths.Resolve()
	if err != nil {
		slog.Error("resolve paths", "err", err)
		return 1
	}
	if err := p.EnsureDirs(); err != nil {
		slog.Error("ensure dirs", "err", err)
		return 1
	}
	slog.Info("paths ready", "base", p.Base, "version", version)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, err := store.Open(ctx, p.DBFile())
	if err != nil {
		slog.Error("open store", "err", err)
		return 1
	}
	defer st.Close()

	imp := &importer.Importer{Store: st, Paths: p}
	hub := importer.NewHub()
	srv := &server.Server{Store: st, Importer: imp, Hub: hub}

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("shutting down")
		shutCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = httpSrv.Shutdown(shutCtx)
		cancel()
	}()

	slog.Info("listening", "addr", "http://"+addr)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("listen", "err", err)
		return 1
	}
	return 0
}
