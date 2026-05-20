package importer

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
}

// imapSession isolates the go-imap client so imapSource is unit-testable with a fake.
type imapSession interface {
	UIDs() ([]imap.UID, error)
	Fetch(uid imap.UID) ([]byte, error)
	Close() error
}

type imapSource struct {
	cfg     IMAPConfig
	dial    func(IMAPConfig) (imapSession, error)
	session imapSession
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

	uids, err := sess.UIDs()
	if err != nil {
		_ = sess.Close()
		return nil, err
	}
	s.session = sess

	items := make([]Item, 0, len(uids))
	for _, uid := range uids {
		u := uid
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
	return items, nil
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
}

func newIMAPSession(cfg IMAPConfig) (imapSession, error) {
	port := cfg.Port
	if port == 0 {
		port = 993
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(port))

	var (
		c   *imapclient.Client
		err error
	)
	if port == 143 {
		c, err = imapclient.DialStartTLS(addr, nil)
	} else {
		c, err = imapclient.DialTLS(addr, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("imap dial: %w", err)
	}

	if err := c.Login(cfg.Username, cfg.Password).Wait(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("imap login: %w", err)
	}

	folder := cfg.Folder
	if folder == "" {
		folder = "INBOX"
	}
	if _, err := c.Select(folder, &imap.SelectOptions{ReadOnly: true}).Wait(); err != nil {
		_ = c.Logout().Wait()
		_ = c.Close()
		return nil, fmt.Errorf("imap select %q: %w", folder, err)
	}

	return &imapClientSession{
		client:      c,
		bodySection: &imap.FetchItemBodySection{Specifier: imap.PartSpecifierNone, Peek: true},
	}, nil
}

func (s *imapClientSession) UIDs() ([]imap.UID, error) {
	data, err := s.client.UIDSearch(&imap.SearchCriteria{}, nil).Wait()
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
