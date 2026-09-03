package parser

import (
	"fmt"
	"html"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/jhillyerd/enmime"

	"github.com/hwhang0917/local-eml/internal/risk"
)

type Parsed struct {
	Subject         string
	From            string
	To              []string
	Cc              []string
	MessageID       string
	ThreadID        string
	Date            time.Time
	BodyText        string
	HTMLAvailable   bool
	AttachmentCount int
	// Risk is the phishing heuristics' findings; empty (not nil) when clean.
	Risk []risk.Reason
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
	p.ThreadID = threadID(env.GetHeader("References"), env.GetHeader("In-Reply-To"), p.MessageID)
	p.To = splitAddrs(env.GetHeader("To"))
	p.Cc = splitAddrs(env.GetHeader("Cc"))
	if dh := env.GetHeader("Date"); dh != "" {
		if t, err := parseDate(dh); err == nil {
			p.Date = t
		}
	}
	p.BodyText = strings.TrimSpace(extractBodyText(env))
	p.Risk = risk.Assess(env)
	return p, nil
}

// threadID returns the conversation key: the first Message-ID in References
// (by RFC 5322 convention the thread root), else In-Reply-To, else the
// message's own Message-ID. Empty when none of the three exist — such a
// message can never be grouped. Angle brackets are stripped so every member
// derives the same key regardless of which header it came from.
func threadID(refs, inReplyTo, msgID string) string {
	for _, h := range []string{refs, inReplyTo, msgID} {
		if f := strings.Fields(h); len(f) > 0 {
			return strings.Trim(f[0], "<>")
		}
	}
	return ""
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
