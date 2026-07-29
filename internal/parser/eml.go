package parser

import (
	"fmt"
	"html"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/jhillyerd/enmime"
)

type Parsed struct {
	Subject         string
	From            string
	To              []string
	Cc              []string
	MessageID       string
	Date            time.Time
	BodyText        string
	HTMLAvailable   bool
	AttachmentCount int
}

func Open(r io.Reader) (*enmime.Envelope, error) {
	return enmime.ReadEnvelope(r)
}

func Parse(r io.Reader) (*Parsed, error) {
	env, err := enmime.ReadEnvelope(r)
	if err != nil {
		return nil, err
	}
	p := &Parsed{
		Subject:   env.GetHeader("Subject"),
		From:      env.GetHeader("From"),
		MessageID: env.GetHeader("Message-ID"),
		// Inlines are cid: images embedded in the HTML body (logos, signatures) —
		// counting them made nearly every marketing email claim an attachment.
		AttachmentCount: len(env.Attachments),
		HTMLAvailable:   env.HTML != "",
	}
	p.To = splitAddrs(env.GetHeader("To"))
	p.Cc = splitAddrs(env.GetHeader("Cc"))
	if dh := env.GetHeader("Date"); dh != "" {
		if t, err := parseDate(dh); err == nil {
			p.Date = t
		}
	}
	p.BodyText = strings.TrimSpace(extractBodyText(env))
	return p, nil
}

func extractBodyText(env *enmime.Envelope) string {
	if env.Text != "" {
		return env.Text
	}
	if env.HTML != "" {
		return htmlToText(env.HTML)
	}
	return ""
}

func splitAddrs(h string) []string {
	if h == "" {
		return nil
	}
	parts := strings.Split(h, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func parseDate(h string) (time.Time, error) {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 -0700 (MST)",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, h); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("could not parse date %q", h)
}

var (
	tagRE   = regexp.MustCompile(`<[^>]*>`)
	spaceRE = regexp.MustCompile(`\s+`)
)

func htmlToText(s string) string {
	s = tagRE.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = spaceRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
