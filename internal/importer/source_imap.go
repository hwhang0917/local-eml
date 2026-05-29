package importer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

type IMAPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Folder   string

	// Sync mode (optional). When Incremental is true and the server's current
	// UIDVALIDITY equals ExpectedUIDValidity, only UIDs strictly greater than
	// SinceUID are searched. On a UIDVALIDITY mismatch we transparently fall
	// back to a full scan since the old state is no longer trustworthy.
	Incremental         bool
	ExpectedUIDValidity uint32
	SinceUID            uint32
}

// SyncResult captures what the source observed during Scan: the current
// UIDVALIDITY of the selected mailbox and the highest UID known to it. The
// caller persists these values so the next run can be incremental.
type SyncResult struct {
	UIDValidity uint32
	MaxUID      uint32
}

// SyncReporter is satisfied by Sources that can report a SyncResult after
// Scan returns. The importer Job calls it via a post-run hook (see runJob).
type SyncReporter interface {
	SyncResult() (SyncResult, bool)
}

// imapSession isolates the go-imap client so imapSource is unit-testable with a fake.
type imapSession interface {
	UIDValidity() uint32
	UIDs(minUID uint32) ([]imap.UID, error)
	Fetch(uid imap.UID) ([]byte, error)
	Close() error
}

type imapSource struct {
	cfg     IMAPConfig
	dial    func(IMAPConfig) (imapSession, error)
	session imapSession

	result SyncResult
	hasRun bool
}

func NewIMAPSource(cfg IMAPConfig) SourceCloser {
	return &imapSource{cfg: cfg, dial: newIMAPSession}
}

func (s *imapSource) folder() string {
	if s.cfg.Folder == "" {
		return "INBOX"
	}
	return s.cfg.Folder
}

func (s *imapSource) Label() string {
	return fmt.Sprintf("imap://%s@%s/%s", s.cfg.Username, s.cfg.Host, s.folder())
}

func (s *imapSource) Scan(_ context.Context) ([]Item, error) {
	sess, err := s.dial(s.cfg)
	if err != nil {
		return nil, err
	}

	uidValidity := sess.UIDValidity()
	var minUID uint32
	if s.cfg.Incremental && uidValidity != 0 && uidValidity == s.cfg.ExpectedUIDValidity {
		minUID = s.cfg.SinceUID + 1
	}

	rawUIDs, err := sess.UIDs(minUID)
	if err != nil {
		_ = sess.Close()
		return nil, err
	}
	// Defensive filter: some servers expand UID N:* even when no message has
	// UID >= N. Drop anything that slipped in below the requested floor.
	uids := rawUIDs[:0]
	for _, u := range rawUIDs {
		if minUID > 0 && uint32(u) < minUID {
			continue
		}
		uids = append(uids, u)
	}
	s.session = sess

	maxUID := s.cfg.SinceUID
	items := make([]Item, 0, len(uids))
	for _, uid := range uids {
		u := uid
		if uint32(u) > maxUID {
			maxUID = uint32(u)
		}
		items = append(items, Item{
			Name: fmt.Sprintf("uid-%d.eml", u),
			Open: func(context.Context) (io.ReadCloser, error) {
				b, err := sess.Fetch(u)
				if err != nil {
					return nil, err
				}
				return io.NopCloser(bytes.NewReader(b)), nil
			},
		})
	}

	s.result = SyncResult{UIDValidity: uidValidity, MaxUID: maxUID}
	s.hasRun = true
	return items, nil
}

func (s *imapSource) SyncResult() (SyncResult, bool) {
	return s.result, s.hasRun && s.result.UIDValidity != 0
}

func (s *imapSource) Close() error {
	if s.session != nil {
		return s.session.Close()
	}
	return nil
}

// imapClientSession is the real adapter over go-imap/v2. It is NOT unit-tested
// (the maintainer smoke-tests it against a real server); it sits behind imapSession.
type imapClientSession struct {
	client      *imapclient.Client
	bodySection *imap.FetchItemBodySection
	uidValidity uint32
}

func newIMAPSession(cfg IMAPConfig) (imapSession, error) {
	port := cfg.Port
	if port == 0 {
		port = 993
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(port))
	log := slog.Default().With(slog.String("addr", addr), slog.String("user", cfg.Username))

	var (
		c   *imapclient.Client
		err error
	)
	if port == 143 {
		log.Info("imap dial starttls")
		c, err = imapclient.DialStartTLS(addr, nil)
	} else {
		log.Info("imap dial tls")
		c, err = imapclient.DialTLS(addr, nil)
	}
	if err != nil {
		log.Error("imap dial failed", slog.String("err", err.Error()))
		return nil, fmt.Errorf("imap dial: %w", err)
	}

	if err := c.Login(cfg.Username, cfg.Password).Wait(); err != nil {
		log.Error("imap login failed", slog.String("err", err.Error()))
		_ = c.Close()
		return nil, fmt.Errorf("imap login: %w", err)
	}
	log.Info("imap logged in")

	folder := cfg.Folder
	if folder == "" {
		folder = "INBOX"
	}
	selectData, err := c.Select(folder, &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		log.Error("imap select failed",
			slog.String("folder", folder), slog.String("err", err.Error()))
		_ = c.Logout().Wait()
		_ = c.Close()
		return nil, fmt.Errorf("imap select %q: %w", folder, err)
	}
	log.Info("imap folder selected",
		slog.String("folder", folder),
		slog.Uint64("uid_validity", uint64(selectData.UIDValidity)),
		slog.Uint64("messages", uint64(selectData.NumMessages)),
	)

	return &imapClientSession{
		client:      c,
		bodySection: &imap.FetchItemBodySection{Specifier: imap.PartSpecifierNone, Peek: true},
		uidValidity: selectData.UIDValidity,
	}, nil
}

func (s *imapClientSession) UIDValidity() uint32 {
	return s.uidValidity
}

func (s *imapClientSession) UIDs(minUID uint32) ([]imap.UID, error) {
	criteria := &imap.SearchCriteria{}
	if minUID > 0 {
		var set imap.UIDSet
		set.AddRange(imap.UID(minUID), 0)
		criteria.UID = []imap.UIDSet{set}
	}
	data, err := s.client.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("imap search: %w", err)
	}
	return data.AllUIDs(), nil
}

func (s *imapClientSession) Fetch(uid imap.UID) ([]byte, error) {
	opts := &imap.FetchOptions{BodySection: []*imap.FetchItemBodySection{s.bodySection}}
	msgs, err := s.client.Fetch(imap.UIDSetNum(uid), opts).Collect()
	if err != nil {
		return nil, fmt.Errorf("imap fetch uid %d: %w", uid, err)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("imap fetch uid %d: no message returned", uid)
	}
	body := msgs[0].FindBodySection(s.bodySection)
	if body == nil {
		return nil, fmt.Errorf("imap fetch uid %d: empty body", uid)
	}
	return body, nil
}

func (s *imapClientSession) Close() error {
	_ = s.client.Logout().Wait()
	return s.client.Close()
}
