package importer

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"net/textproto"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
	pst "github.com/mooijtech/go-pst/v6/pkg"
	"github.com/mooijtech/go-pst/v6/pkg/properties"
)

// pstSource reads an Outlook .pst archive. PST stores messages as MAPI
// property bags rather than RFC 822, so each item is rebuilt into an .eml:
// original transport headers when Outlook kept them, plus the plain/HTML
// body and attachments. Scan walks every folder collecting message ids so
// progress gets a real total; Open then loads one message at a time.
type pstSource struct {
	path string
	f    *os.File
	pf   *pst.File
}

func NewPstSource(path string) SourceCloser { return &pstSource{path: path} }

func (s *pstSource) Label() string { return "pst archive" }

func (s *pstSource) Scan(_ context.Context) ([]Item, error) {
	f, err := os.Open(s.path)
	if err != nil {
		return nil, fmt.Errorf("open pst: %w", err)
	}
	pf, err := pst.New(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("read pst: %w", err)
	}
	s.f, s.pf = f, pf

	var ids []pst.Identifier
	err = pf.WalkFolders(func(folder *pst.Folder) error {
		it, err := folder.GetMessageIterator()
		if err != nil {
			// go-pst reports an empty folder as an error; nothing to import there.
			return nil
		}
		for it.Next() {
			// ponytail: only mail items; contacts, appointments and tasks are
			// not emails and have no .eml shape to land in.
			if _, ok := it.Value().Properties.(*properties.Message); ok {
				ids = append(ids, it.Value().Identifier)
			}
		}
		return it.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("scan pst: %w", err)
	}

	base := filepath.Base(s.path)
	items := make([]Item, len(ids))
	for i, id := range ids {
		items[i] = Item{
			Name: fmt.Sprintf("%s#%05d.eml", base, i+1),
			Open: func(context.Context) (io.ReadCloser, error) {
				m, err := pf.GetMessage(id)
				if err != nil {
					return nil, fmt.Errorf("read pst message: %w", err)
				}
				var buf bytes.Buffer
				if err := writePstEML(&buf, m); err != nil {
					return nil, err
				}
				return io.NopCloser(&buf), nil
			},
		}
	}
	return items, nil
}

func (s *pstSource) Close() error {
	if s.pf != nil {
		s.pf.Cleanup()
	}
	if s.f != nil {
		return s.f.Close()
	}
	return nil
}

// writePstEML rebuilds one PST message as RFC 822. Transport headers (when
// Outlook kept them) carry the real From/To/Date/Message-ID; MAPI properties
// fill whatever is missing, which is the normal case for Exchange-internal
// mail where the sender is just a display name.
func writePstEML(w io.Writer, m *pst.Message) error {
	p := m.Properties.(*properties.Message)

	var h mail.Header
	transport := parseTransportHeaders(p.GetTransportMessageHeaders())
	// Sorted so the rebuilt bytes are stable; map order would defeat dedup.
	for _, k := range slices.Sorted(maps.Keys(transport)) {
		if isBodyHeader(k) {
			continue
		}
		for _, v := range transport[k] {
			h.Add(k, v)
		}
	}
	if !h.Has("Subject") && p.GetSubject() != "" {
		h.SetSubject(p.GetSubject())
	}
	if !h.Has("From") {
		setNameAddr(&h, "From", p.GetSentRepresentingName(), p.GetSentRepresentingEmailAddress())
	}
	if !h.Has("To") && p.GetDisplayTo() != "" {
		h.SetText("To", p.GetDisplayTo())
	}
	if !h.Has("Cc") && p.GetDisplayCc() != "" {
		h.SetText("Cc", p.GetDisplayCc())
	}
	if !h.Has("Date") {
		if ns := p.GetClientSubmitTime(); ns != 0 {
			h.SetDate(time.Unix(0, ns))
		} else if ns := p.GetMessageDeliveryTime(); ns != 0 {
			h.SetDate(time.Unix(0, ns))
		}
	}
	if !h.Has("Message-Id") && p.GetInternetMessageId() != "" {
		h.Set("Message-Id", p.GetInternetMessageId())
	}
	if !h.Has("In-Reply-To") && p.GetInReplyToId() != "" {
		h.Set("In-Reply-To", p.GetInReplyToId())
	}
	if !h.Has("References") && p.GetInternetReferences() != "" {
		h.Set("References", p.GetInternetReferences())
	}

	// Boundaries are derived from the message id rather than random so the
	// same message rebuilds to the same bytes and the hash-based dedup holds
	// across re-imports of one archive.
	boundary := fmt.Sprintf("=_local-eml-pst-%d-", m.Identifier)
	h.SetContentType("multipart/mixed", map[string]string{"boundary": boundary + "mixed"})
	mw, err := message.CreateWriter(w, h.Header)
	if err != nil {
		return fmt.Errorf("write eml header: %w", err)
	}
	var alt message.Header
	alt.SetContentType("multipart/alternative", map[string]string{"boundary": boundary + "alt"})
	aw, err := mw.CreatePart(alt)
	if err != nil {
		return err
	}
	// ponytail: RTF-only bodies come out empty; add RTF de-encapsulation
	// (MS-OXRTFEX) if real archives turn out to lack PR_BODY/PR_HTML.
	text, html := p.GetBody(), p.GetBodyHtml()
	if text != "" || html == "" {
		if err := writePart(aw, "text/plain", text); err != nil {
			return err
		}
	}
	if html != "" {
		if err := writePart(aw, "text/html", html); err != nil {
			return err
		}
	}
	if err := aw.Close(); err != nil {
		return err
	}

	if has, _ := m.HasAttachments(); has {
		it, err := m.GetAttachmentIterator()
		if err != nil {
			return fmt.Errorf("read pst attachments: %w", err)
		}
		for it.Next() {
			if err := writeAttachment(mw, it.Value()); err != nil {
				return err
			}
		}
		if err := it.Err(); err != nil {
			return fmt.Errorf("read pst attachments: %w", err)
		}
	}
	return mw.Close()
}

func writePart(aw *message.Writer, contentType, body string) error {
	var ph message.Header
	ph.SetContentType(contentType, map[string]string{"charset": "utf-8"})
	ph.Set("Content-Transfer-Encoding", "quoted-printable")
	pw, err := aw.CreatePart(ph)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(pw, body); err != nil {
		return err
	}
	return pw.Close()
}

func writeAttachment(mw *message.Writer, a *pst.Attachment) error {
	name := a.GetAttachLongFilename()
	if name == "" {
		name = a.GetAttachFilename()
	}
	if name == "" {
		name = "attachment"
	}
	ct := a.GetAttachMimeTag()
	if ct == "" {
		ct = "application/octet-stream"
	}
	var ah mail.AttachmentHeader
	ah.SetFilename(name)
	ah.SetContentType(ct, nil)
	ah.Set("Content-Transfer-Encoding", "base64")
	if cid := strings.Trim(a.GetAttachContentId(), "<>"); cid != "" {
		ah.Set("Content-Id", "<"+cid+">")
	}
	aw, err := mw.CreatePart(ah.Header)
	if err != nil {
		return err
	}
	if _, err := a.WriteTo(aw); err != nil {
		return fmt.Errorf("read pst attachment %q: %w", name, err)
	}
	return aw.Close()
}

// parseTransportHeaders is best-effort: a malformed line keeps whatever was
// parsed before it rather than dropping the whole block.
func parseTransportHeaders(raw string) textproto.MIMEHeader {
	if raw == "" {
		return nil
	}
	r := textproto.NewReader(bufio.NewReader(strings.NewReader(raw + "\r\n\r\n")))
	h, _ := r.ReadMIMEHeader()
	return h
}

// isBodyHeader drops the MIME framing of the original message: the body is
// regenerated, so those headers would describe parts that no longer exist.
func isBodyHeader(k string) bool {
	return strings.HasPrefix(k, "Content-") || k == "Mime-Version"
}

// setNameAddr writes a proper address when there is one; Exchange-internal
// mail only has a display name (address type EX), which goes in as text.
func setNameAddr(h *mail.Header, key, name, addr string) {
	switch {
	case strings.Contains(addr, "@"):
		h.SetAddressList(key, []*mail.Address{{Name: name, Address: addr}})
	case name != "":
		h.SetText(key, name)
	case addr != "":
		h.SetText(key, addr)
	}
}
