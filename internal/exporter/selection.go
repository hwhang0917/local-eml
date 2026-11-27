package exporter

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"net/mail"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/emersion/go-mbox"

	"github.com/hwhang0917/local-eml/internal/store"
)

// A selection export hands chosen messages to someone else: a lawyer, an
// accountant, another mail app. Unlike the full-library zip it never carries
// the database, which holds every other message's metadata too.

// IndexName is the spreadsheet-friendly list of a selection zip's contents.
const IndexName = "index.csv"

// maxSubjectRunes bounds the subject part of an entry name, so paths stay
// well inside Windows' 260-character limit once extracted.
const maxSubjectRunes = 60

// WriteSelectionZip streams the given messages as .eml files plus index.csv.
// Entries are named date first, so a file browser lists them in order.
func (e *Exporter) WriteSelectionZip(ctx context.Context, w io.Writer, emails []store.Email) (written, skipped int, err error) {
	zw := zip.NewWriter(w)
	defer func() {
		if cerr := zw.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	type row struct {
		name string
		em   store.Email
	}
	rows := make([]row, 0, len(emails))
	for _, em := range emails {
		if err := ctx.Err(); err != nil {
			return written, skipped, err
		}
		name := selectionEntryName(em)
		if werr := writeOne(zw, e.Paths.BlobFor(em.SHA256), name); werr != nil {
			slog.Warn("selection zip entry failed",
				slog.String("sha256", em.SHA256), slog.String("err", werr.Error()))
			skipped++
			continue
		}
		rows = append(rows, row{name, em})
		written++
	}

	iw, err := zw.CreateHeader(&zip.FileHeader{Name: IndexName, Method: zip.Deflate, Modified: time.Now()})
	if err != nil {
		return written, skipped, err
	}
	// A BOM so Excel opens the UTF-8 file with Korean and other non-Latin
	// text intact instead of guessing the system code page.
	if _, err := io.WriteString(iw, "\uFEFF"); err != nil {
		return written, skipped, err
	}
	cw := csv.NewWriter(iw)
	_ = cw.Write([]string{"file", "date", "from", "to", "cc", "subject", "attachments"})
	for _, r := range rows {
		_ = cw.Write([]string{
			r.name,
			formatIndexDate(r.em.SentAt),
			r.em.FromAddr,
			strings.Join(r.em.ToAddrs, ", "),
			strings.Join(r.em.CcAddrs, ", "),
			r.em.Subject,
			fmt.Sprint(r.em.AttachmentCount),
		})
	}
	cw.Flush()
	return written, skipped, cw.Error()
}

// WriteMbox streams the given messages as one mbox file, which Thunderbird,
// Apple Mail and Local Eml itself can open.
func (e *Exporter) WriteMbox(ctx context.Context, w io.Writer, emails []store.Email) (written, skipped int, err error) {
	mw := mbox.NewWriter(w)
	defer func() {
		if cerr := mw.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	for _, em := range emails {
		if err := ctx.Err(); err != nil {
			return written, skipped, err
		}
		f, oerr := os.Open(e.Paths.BlobFor(em.SHA256))
		if oerr != nil {
			slog.Warn("mbox entry failed",
				slog.String("sha256", em.SHA256), slog.String("err", oerr.Error()))
			skipped++
			continue
		}
		dst, cerr := mw.CreateMessage(envelopeSender(em.FromAddr), em.SentAt)
		if cerr != nil {
			f.Close()
			return written, skipped, cerr
		}
		_, cerr = io.Copy(dst, f)
		f.Close()
		if cerr != nil {
			// The message is half written; the stream can't be repaired.
			return written, skipped, cerr
		}
		written++
	}
	return written, skipped, nil
}

// envelopeSender is the bare address for an mbox "From " line, which can't
// hold a display name or spaces.
func envelopeSender(from string) string {
	if a, err := mail.ParseAddress(from); err == nil && a.Address != "" {
		return a.Address
	}
	if f := strings.Fields(from); len(f) == 1 && strings.Contains(f[0], "@") {
		return strings.Trim(f[0], "<>")
	}
	return "MAILER-DAEMON"
}

func formatIndexDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}

// selectionEntryName is "<date>_<sha8>_<subject>.eml". The short sha keeps
// names unique when two messages share a day and subject.
func selectionEntryName(em store.Email) string {
	date := "undated"
	if !em.SentAt.IsZero() {
		date = em.SentAt.Local().Format("2006-01-02")
	}
	name := date + "_" + shortSHA(em.SHA256)
	if subj := safeSubject(em.Subject); subj != "" {
		name += "_" + subj
	}
	return name + ".eml"
}

// safeSubject keeps a subject usable as a file name on every OS: no path
// separators, no characters Windows refuses, no control characters, and no
// trailing dots or spaces.
func safeSubject(s string) string {
	var b strings.Builder
	n := 0
	space := false
	for _, r := range s {
		if n >= maxSubjectRunes {
			break
		}
		switch {
		case strings.ContainsRune(`<>:"/\|?*`, r), unicode.IsControl(r):
			r = '_'
		case unicode.IsSpace(r):
			if space {
				continue
			}
			r = ' '
		}
		space = r == ' '
		b.WriteRune(r)
		n++
	}
	return strings.TrimRight(strings.TrimSpace(b.String()), ". ")
}
