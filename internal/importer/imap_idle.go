package importer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// ErrIdleUnsupported is returned by IdleWatcher.Run when the server's
// CAPABILITY list has no IDLE. The caller keeps polling that profile instead.
var ErrIdleUnsupported = errors.New("imap server does not support IDLE")

// IdleState is what an IdleWatcher is doing right now, as the profile cards
// show it.
type IdleState string

const (
	IdleConnecting  IdleState = "connecting"  // dialing or logging in
	IdleLive        IdleState = "live"        // IDLE is running; new mail is pushed
	IdleRetrying    IdleState = "retrying"    // the connection dropped; waiting to redial
	IdleUnsupported IdleState = "unsupported" // the server has no IDLE; polling only
)

// Defaults for IdleWatcher. RFC 2177 has servers drop an IDLE after 30
// minutes, and home routers often forget a quiet connection well before that,
// so IDLE is re-issued every 20 minutes. Backoff starts small so a brief
// network blip costs seconds, and caps where a server that keeps refusing us
// (a changed password, say) is retried a few times an hour.
const (
	DefaultIdleRestart = 20 * time.Minute
	DefaultIdleMinWait = 5 * time.Second
	DefaultIdleMaxWait = 15 * time.Minute

	// idleStopTimeout bounds how long ending IDLE may take. A connection a
	// NAT silently dropped never answers DONE; past this it is treated as gone.
	idleStopTimeout = 30 * time.Second
	// idleLogoutTimeout bounds the polite LOGOUT before the socket is closed.
	idleLogoutTimeout = 5 * time.Second
)

// idleConn is one logged-in connection with the watched folder selected. It
// isolates go-imap so the watcher's loop is unit-testable with a fake.
type idleConn interface {
	// SupportsIdle reports whether the server advertised IDLE (or IMAP4rev2,
	// which includes it).
	SupportsIdle() bool
	// Wait runs one IDLE until the server reports a change to the folder's
	// message count (changed is true), d elapses, or ctx ends. An error means
	// the connection is no longer usable.
	Wait(ctx context.Context, d time.Duration) (changed bool, err error)
	Close() error
}

// IdleWatcher keeps one IMAP connection idling on a folder and calls OnChange
// whenever the server says new mail arrived. It also calls OnChange right
// after every (re)connect, so mail that came in while it was away is fetched
// too. The fetching itself is the caller's: the watcher only rings the bell.
type IdleWatcher struct {
	Config   IMAPConfig
	OnChange func()
	// OnState, when set, is told every state change.
	OnState func(IdleState)
	Logger  *slog.Logger

	// Zero values pick the Default* constants.
	Restart time.Duration
	MinWait time.Duration
	MaxWait time.Duration

	dial func(IMAPConfig) (idleConn, error)
}

// Run watches until ctx ends (returning ctx.Err()) or the server turns out
// not to support IDLE (returning ErrIdleUnsupported). Connection failures are
// retried with exponential backoff and never end the loop.
func (w *IdleWatcher) Run(ctx context.Context) error {
	dial := w.dial
	if dial == nil {
		dial = dialIdle
	}
	log := w.Logger
	if log == nil {
		log = slog.Default()
	}
	log = log.With(slog.String("folder", w.Config.Mailbox()))

	wait := or(w.MinWait, DefaultIdleMinWait)
	for {
		w.setState(IdleConnecting)
		conn, err := dial(w.Config)
		if err == nil {
			if !conn.SupportsIdle() {
				_ = conn.Close()
				log.Info("imap idle unsupported; polling only")
				w.setState(IdleUnsupported)
				return ErrIdleUnsupported
			}
			log.Info("imap idle live")
			w.setState(IdleLive)
			w.change() // catch up on whatever arrived while we were away
			err = w.watch(ctx, conn, &wait)
			_ = conn.Close()
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		delay := jitter(wait)
		log.Warn("imap idle dropped; reconnecting",
			slog.String("err", errString(err)), slog.Duration("in", delay))
		w.setState(IdleRetrying)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		wait = min(wait*2, or(w.MaxWait, DefaultIdleMaxWait))
	}
}

// watch idles on conn until it fails or ctx ends. A full IDLE round trip
// proves the connection healthy, so it resets the backoff.
func (w *IdleWatcher) watch(ctx context.Context, conn idleConn, wait *time.Duration) error {
	restart := or(w.Restart, DefaultIdleRestart)
	for {
		changed, err := conn.Wait(ctx, restart)
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
		*wait = or(w.MinWait, DefaultIdleMinWait)
		if changed {
			w.change()
		}
	}
}

func (w *IdleWatcher) change() {
	if w.OnChange != nil {
		w.OnChange()
	}
}

func (w *IdleWatcher) setState(s IdleState) {
	if w.OnState != nil {
		w.OnState(s)
	}
}

func or(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

// jitter spreads d by up to a fifth either way, so profiles that dropped
// together (the network went away) don't all redial in the same instant.
func jitter(d time.Duration) time.Duration {
	spread := int64(d) / 5
	if spread <= 0 {
		return d
	}
	return d - time.Duration(spread) + time.Duration(rand.Int64N(2*spread+1))
}

func errString(err error) string {
	if err == nil {
		return "closed"
	}
	return err.Error()
}

// imapIdleClient is the real idleConn over go-imap/v2. Like
// imapClientSession it is smoke-tested against real servers, not unit-tested.
type imapIdleClient struct {
	client  *imapclient.Client
	changed chan struct{}
	idle    bool
}

func dialIdle(cfg IMAPConfig) (idleConn, error) {
	// The server announces new mail as an untagged EXISTS while IDLE runs.
	// Buffered by one: a burst of arrivals needs only one fetch.
	changed := make(chan struct{}, 1)
	opts := &imapclient.Options{
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			Mailbox: func(d *imapclient.UnilateralDataMailbox) {
				if d.NumMessages == nil {
					return
				}
				select {
				case changed <- struct{}{}:
				default:
				}
			},
		},
	}
	c, log, err := dialIMAP(cfg, opts)
	if err != nil {
		return nil, err
	}
	ic := &imapIdleClient{client: c, changed: changed, idle: c.Caps().Has(imap.CapIdle)}
	if !ic.idle {
		return ic, nil
	}

	folder := cfg.Mailbox()
	if _, err := c.Select(folder, &imap.SelectOptions{ReadOnly: true}).Wait(); err != nil {
		log.Error("imap idle select failed", slog.String("folder", folder), slog.String("err", err.Error()))
		_ = ic.Close()
		return nil, fmt.Errorf("imap select %q: %w", folder, err)
	}
	// SELECT reports the current count too; that is not news.
	select {
	case <-changed:
	default:
	}
	return ic, nil
}

func (c *imapIdleClient) SupportsIdle() bool { return c.idle }

func (c *imapIdleClient) Wait(ctx context.Context, d time.Duration) (bool, error) {
	cmd, err := c.client.Idle()
	if err != nil {
		return false, fmt.Errorf("imap idle: %w", err)
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	changed := false
	select {
	case <-c.changed:
		changed = true
	case <-timer.C:
	case <-ctx.Done():
	case <-c.client.Closed():
		return false, errors.New("imap idle: connection closed by server")
	}

	if err := cmd.Close(); err != nil {
		return changed, fmt.Errorf("imap idle stop: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return changed, fmt.Errorf("imap idle: %w", err)
		}
		return changed, nil
	case <-time.After(idleStopTimeout):
		return changed, errors.New("imap idle: server stopped answering")
	}
}

func (c *imapIdleClient) Close() error {
	done := make(chan struct{})
	go func() {
		_ = c.client.Logout().Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(idleLogoutTimeout):
	}
	return c.client.Close()
}
