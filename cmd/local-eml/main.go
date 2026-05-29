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
	"path/filepath"
	"syscall"
	"time"

	"github.com/hwhang0917/local-eml/internal/exporter"
	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/paths"
	"github.com/hwhang0917/local-eml/internal/secret"
	"github.com/hwhang0917/local-eml/internal/server"
	"github.com/hwhang0917/local-eml/internal/store"
)

var version = "dev"

// syncInterval lets ops tune the IMAP background poll without rebuilding.
// LOCAL_EML_SYNC_INTERVAL: a Go duration (e.g. "5m", "30s") or "off" to disable.
// Default: 10 minutes — small enough to feel like "incoming mail," large enough
// to be polite to remote IMAP servers.
func syncInterval() time.Duration {
	switch v := os.Getenv("LOCAL_EML_SYNC_INTERVAL"); v {
	case "":
		return 10 * time.Minute
	case "off", "0", "disabled":
		return 0
	default:
		d, err := time.ParseDuration(v)
		if err != nil || d < 0 {
			slog.Warn("bad LOCAL_EML_SYNC_INTERVAL, using default",
				slog.String("value", v))
			return 10 * time.Minute
		}
		return d
	}
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		os.Exit(runServe(os.Args[2:]))
	case "install":
		os.Exit(runInstall(os.Args[2:]))
	case "uninstall":
		os.Exit(runUninstall(os.Args[2:]))
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
	fmt.Fprintln(os.Stderr, "usage: local-eml <command> [flags]")
	fmt.Fprintln(os.Stderr, "  serve [--port 7878]        run the local web server (loopback only)")
	fmt.Fprintln(os.Stderr, "  install [-y|--yes]         register as a background service (systemd/launchd/svc)")
	fmt.Fprintln(os.Stderr, "  uninstall [-y|--yes]       stop and unregister the background service")
	fmt.Fprintln(os.Stderr, "  version | -V | --version")
}

func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 7878, "TCP port to listen on (loopback only)")
	_ = fs.Parse(args)

	// Bootstrap stderr-only logger for very early errors before paths exist.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	p, err := paths.Resolve()
	if err != nil {
		slog.Error("resolve paths", "err", err)
		return 1
	}
	if err := p.EnsureDirs(); err != nil {
		slog.Error("ensure dirs", "err", err)
		return 1
	}

	logger, closeLog := setupLogger(p)
	defer closeLog()
	slog.SetDefault(logger)
	slog.Info("paths ready", "base", p.Base, "log", filepath.Join(p.Logs, "local-eml.log"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, err := store.Open(ctx, p.DBFile())
	if err != nil {
		slog.Error("open store", "err", err)
		return 1
	}
	defer st.Close()

	sec, err := secret.Open(p.KeyFile())
	if err != nil {
		slog.Error("open secret store", "err", err)
		return 1
	}
	imp := &importer.Importer{Store: st, Paths: p}
	hub := importer.NewHub()
	exp := &exporter.Exporter{Store: st, Paths: p, Hub: hub}
	canc := importer.NewCanceller()
	srv := &server.Server{
		Store: st, Importer: imp, Exporter: exp,
		Hub: hub, Canceller: canc, Secret: sec,
	}
	srv.StartIMAPSyncer(ctx, syncInterval())

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
