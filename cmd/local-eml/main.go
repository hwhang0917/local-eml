package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/kardianos/service"

	"github.com/hwhang0917/local-eml/internal/exporter"
	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/paths"
	"github.com/hwhang0917/local-eml/internal/prefs"
	"github.com/hwhang0917/local-eml/internal/secret"
	"github.com/hwhang0917/local-eml/internal/server"
	"github.com/hwhang0917/local-eml/internal/store"
	"github.com/hwhang0917/local-eml/internal/tempsession"
)

var version = "dev"

const defaultPort = 7878

// idleGrace is how long app mode keeps serving with no /healthz pings before
// exiting. The UI pings every 30s, but browsers throttle background-tab timers
// to ~1/min, so anything under a couple of minutes would false-positive.
const idleGrace = 3 * time.Minute

// ephemeralEnv requests a temporary session without a flag, for launchers
// that can set the environment but not arguments.
const ephemeralEnv = "LOCAL_EML_EPHEMERAL"

func ephemeralFromEnv() bool {
	v := os.Getenv(ephemeralEnv)
	return v == "1" || strings.EqualFold(v, "true")
}

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

// resolveSyncInterval prefers the interval saved from the UI over the env var:
// a knob the user can see and change must not be silently trumped by one they
// can't.
func resolveSyncInterval(ctx context.Context, st *store.Store) time.Duration {
	v, err := st.GetSetting(ctx, server.SyncIntervalSettingKey)
	if err != nil || v == "" {
		return syncInterval()
	}
	secs, err := strconv.Atoi(v)
	if err != nil || secs < 0 {
		slog.Warn("bad stored sync interval, using default", slog.String("value", v))
		return syncInterval()
	}
	return time.Duration(secs) * time.Second
}

// psnPrefix marks the process serial number macOS LaunchServices may pass to
// an app it launches (-psn_0_12345). It carries nothing we use.
const psnPrefix = "-psn_"

// isAppLaunch reports whether args start the browser-window app, the mode a
// double-click runs; the terminal commands keep their console.
func isAppLaunch(args []string) bool {
	if len(args) == 0 {
		return true
	}
	switch args[0] {
	case "app", "--ephemeral", "-ephemeral":
		return true
	}
	return strings.HasPrefix(args[0], psnPrefix)
}

func main() {
	if isAppLaunch(os.Args[1:]) && relaunchWithoutConsole() {
		return
	}
	if len(os.Args) < 2 || strings.HasPrefix(os.Args[1], psnPrefix) {
		// Double-clicked (or bare `local-eml`): behave like an app.
		os.Exit(runApp(nil))
	}
	switch os.Args[1] {
	case "serve":
		os.Exit(runServe(os.Args[2:]))
	case "app":
		os.Exit(runApp(os.Args[2:]))
	case "--ephemeral", "-ephemeral":
		os.Exit(runApp(os.Args[1:]))
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
	fmt.Fprintln(os.Stderr, "usage: local-eml [command] [flags]")
	fmt.Fprintln(os.Stderr, "  app [--ephemeral]          open in a browser window; server stops when the window closes (default)")
	fmt.Fprintln(os.Stderr, "  serve [--port 7878] [--ephemeral]")
	fmt.Fprintln(os.Stderr, "                             run the local web server (loopback only)")
	fmt.Fprintln(os.Stderr, "  --ephemeral                temporary session: data lives in a temp folder deleted on exit")
	fmt.Fprintf(os.Stderr, "                             (or set %s=1)\n", ephemeralEnv)
	fmt.Fprintln(os.Stderr, "  install [-y|--yes] [--port 7878]")
	fmt.Fprintln(os.Stderr, "                             register as a background service (systemd/launchd/svc)")
	fmt.Fprintln(os.Stderr, "  uninstall [-y|--yes]       stop and unregister the background service")
	fmt.Fprintln(os.Stderr, "  version | -V | --version")
}

func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", defaultPort, "TCP port to listen on (loopback only)")
	ephemeral := fs.Bool("ephemeral", false, "temporary session: data is deleted on exit")
	_ = fs.Parse(args)
	temporary := *ephemeral || ephemeralFromEnv()
	// The service has no "close" to clean up after, so a temporary service
	// would be one that silently deletes its library on every reboot.
	if temporary && !service.Interactive() {
		fmt.Fprintln(os.Stderr, "ignoring --ephemeral: not available when running as a service")
		temporary = false
	}
	return serve(serveOptions{port: *port, temporary: temporary})
}

// runApp gives the double-click experience: reuse a running server if there is
// one, else serve on an ephemeral port; either way open a browser window.
func runApp(args []string) int {
	fs := flag.NewFlagSet("app", flag.ExitOnError)
	ephemeral := fs.Bool("ephemeral", false, "temporary session: data is deleted on exit")
	_ = fs.Parse(args)

	temporary := *ephemeral || ephemeralFromEnv() || temporaryLaunchesPref()
	// A temporary session must never attach to a running server: that would
	// be the service, working on the real library.
	if !temporary && probeExisting(defaultPort) {
		openWindow(fmt.Sprintf("http://127.0.0.1:%d", defaultPort))
		return 0
	}
	return serve(serveOptions{appMode: true, temporary: temporary})
}

// temporaryLaunchesPref reads the "double-click sessions are temporary"
// switch. Unreadable prefs fall back to a normal launch, the same as before
// the switch existed, rather than refusing to start.
func temporaryLaunchesPref() bool {
	p, err := paths.Resolve()
	if err != nil {
		return false
	}
	pr, err := prefs.Load(p.PrefsFile())
	if err != nil {
		fmt.Fprintf(os.Stderr, "ignoring unreadable %s: %v\n", p.PrefsFile(), err)
		return false
	}
	return pr.TemporaryLaunches
}

type serveOptions struct {
	port      int
	appMode   bool
	temporary bool
}

// serve runs the server on either the persistent library or, for a temporary
// session, a fresh throwaway folder that is removed once the server has
// stopped and closed every file in it.
func serve(opts serveOptions) int {
	// Bootstrap stderr-only logger for very early errors before paths exist.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	persistent, err := paths.Resolve()
	if err != nil {
		slog.Error("resolve paths", "err", err)
		return 1
	}
	// Every launch clears what crashed temporary sessions left behind.
	for _, dir := range tempsession.Sweep(os.TempDir()) {
		slog.Info("removed leftover temporary session", "dir", dir)
	}
	if !opts.temporary {
		return run(persistent, persistent.PrefsFile(), opts)
	}

	dir, err := tempsession.Create(os.TempDir())
	if err != nil {
		slog.Error("create temporary session", "err", err)
		return 1
	}
	code := run(paths.At(dir), persistent.PrefsFile(), opts)
	// The log file was inside dir and is closed now; report to stderr only.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := tempsession.Remove(dir); err != nil {
		slog.Error("delete temporary session; it will be retried on next launch", "dir", dir, "err", err)
		return 1
	}
	slog.Info("temporary session deleted", "dir", dir)
	return code
}

// run serves on p until shutdown. Its defers close the database and the log,
// which serve relies on before deleting a temporary folder.
func run(p paths.Paths, prefsFile string, opts serveOptions) int {
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
		Version:            version,
		RunningInteractive: service.Interactive(),
		TemporarySession:   opts.temporary,
		PrefsFile:          prefsFile,
	}
	srv.StartIMAPSyncer(ctx, resolveSyncInterval(ctx, st))

	// One-time data repairs (attachment counts, thread ids), each gated on
	// PRAGMA user_version; run in the background so a large library doesn't
	// delay startup.
	go func() {
		if n, err := imp.BackfillAttachmentCounts(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Warn("attachment count backfill", "err", err)
		} else if n > 0 {
			slog.Info("attachment count backfill done", "updated", n)
		}
		if n, err := imp.BackfillThreadIDs(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Warn("thread id backfill", "err", err)
		} else if n > 0 {
			slog.Info("thread id backfill done", "updated", n)
		}
		if n, err := imp.BackfillRisk(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Warn("phishing risk backfill", "err", err)
		} else if n > 0 {
			slog.Info("phishing risk backfill done", "assessed", n)
		}
	}()

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", opts.port))
	if err != nil {
		slog.Error("listen", "err", err)
		return 1
	}
	url := "http://" + ln.Addr().String()

	handler := http.Handler(srv.Router())
	httpSrv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	var shutdownOnce sync.Once
	shutdownDone := make(chan struct{})
	shutdown := func(reason string) {
		shutdownOnce.Do(func() {
			defer close(shutdownDone)
			slog.Info("shutting down", "reason", reason)
			shutCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
			defer c()
			_ = httpSrv.Shutdown(shutCtx)
			// Jobs run on their own contexts. A temporary folder can only be
			// deleted once they stop writing into it.
			if opts.temporary {
				canc.CancelAll()
			}
			cancel()
		})
	}

	if opts.appMode {
		lastSeen := &atomic.Int64{}
		lastSeen.Store(time.Now().UnixNano())
		httpSrv.Handler = touchHealth(handler, lastSeen)
		go watchIdle(ctx, lastSeen, shutdown)
		go openWindow(url)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		shutdown("signal")
	}()

	slog.Info("listening", "addr", url)
	if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("serve", "err", err)
		return 1
	}
	// Serve returns as soon as Shutdown begins. Wait for it to finish so the
	// deferred closes, and a temporary folder's deletion, come after it.
	<-shutdownDone
	return 0
}

// touchHealth stamps lastSeen on every /healthz hit. The UI polls it every 30s
// while a tab is open, so it doubles as an "is anyone still looking" heartbeat.
func touchHealth(next http.Handler, lastSeen *atomic.Int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			lastSeen.Store(time.Now().UnixNano())
		}
		next.ServeHTTP(w, r)
	})
}

func watchIdle(ctx context.Context, lastSeen *atomic.Int64, shutdown func(string)) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if time.Since(time.Unix(0, lastSeen.Load())) > idleGrace {
				shutdown("no client pings")
				return
			}
		}
	}
}
