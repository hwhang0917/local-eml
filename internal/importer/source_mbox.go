package importer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/emersion/go-mbox"
)

// mboxSource streams messages out of an mbox archive (Google Takeout,
// Thunderbird). Scan counts messages in a first pass so progress gets a real
// total, then reopens the file and hands each message out in order.
type mboxSource struct {
	path string
	f    *os.File
	mr   *mbox.Reader
}

func NewMboxSource(path string) SourceCloser { return &mboxSource{path: path} }

func (s *mboxSource) Label() string { return "mbox archive" }

func (s *mboxSource) Scan(_ context.Context) ([]Item, error) {
	f, err := os.Open(s.path)
	if err != nil {
		return nil, fmt.Errorf("open mbox: %w", err)
	}
	n := 0
	mr := mbox.NewReader(f)
	for {
		m, err := mr.NextMessage()
		if err == io.EOF {
			break
		}
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("scan mbox: %w", err)
		}
		if _, err := io.Copy(io.Discard, m); err != nil {
			f.Close()
			return nil, fmt.Errorf("scan mbox: %w", err)
		}
		n++
	}
	f.Close()

	if s.f, err = os.Open(s.path); err != nil {
		return nil, fmt.Errorf("reopen mbox: %w", err)
	}
	s.mr = mbox.NewReader(s.f)

	base := filepath.Base(s.path)
	items := make([]Item, n)
	for i := range items {
		items[i] = Item{
			Name: fmt.Sprintf("%s#%05d.eml", base, i+1),
			// ponytail: each Open just advances the shared reader, so items must
			// be opened in order — which is how the job driver works. Switch to
			// byte-offset SectionReaders if a concurrent driver ever appears.
			Open: func(context.Context) (io.ReadCloser, error) {
				m, err := s.mr.NextMessage()
				if err != nil {
					return nil, fmt.Errorf("read mbox message: %w", err)
				}
				return io.NopCloser(m), nil
			},
		}
	}
	return items, nil
}

func (s *mboxSource) Close() error {
	if s.f != nil {
		return s.f.Close()
	}
	return nil
}
