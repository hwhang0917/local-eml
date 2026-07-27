package sanitize

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

type Options struct {
	CIDBaseURL string
	ShowRemote bool
	Lang       string
}

// Blocked remote images used to collapse to a 1x1 transparent GIF, which read
// as a broken message rather than a deliberate privacy choice. The replacement
// is a self-describing SVG: a crossed-out image glyph plus a short label, so
// the reason is legible without opening the network panel. Inline SVG keeps it
// a data: URI, which the viewer's CSP already permits under img-src.
const blockedPlaceholderTemplate = `<svg xmlns="http://www.w3.org/2000/svg" width="260" height="90">` +
	`<rect x="0.5" y="0.5" width="259" height="89" rx="6" fill="#f4f4f5" stroke="#d4d4d8" stroke-dasharray="4 3"/>` +
	`<g transform="translate(18,29)" fill="none" stroke="#a1a1aa" stroke-width="1.8" stroke-linecap="round">` +
	`<rect x="1" y="1" width="30" height="24" rx="3"/>` +
	`<circle cx="10" cy="9" r="3"/><path d="M3 22l9-8 7 6 5-4 7 6"/><path d="M2 2l28 22"/></g>` +
	`<text x="62" y="41" font-family="system-ui,-apple-system,Segoe UI,sans-serif" font-size="12" font-weight="600" fill="#3f3f46">%s</text>` +
	`<text x="62" y="59" font-family="system-ui,-apple-system,Segoe UI,sans-serif" font-size="11" fill="#71717a">%s</text>` +
	`</svg>`

var blockedLabels = map[string][2]string{
	"ko": {"외부 이미지 차단됨", "안전을 위해 차단했어요"},
	"en": {"Remote image blocked", "Hidden to protect your privacy"},
}

func blockedPlaceholder(lang string) string {
	labels, ok := blockedLabels[lang]
	if !ok {
		labels = blockedLabels["en"]
	}
	svg := fmt.Sprintf(blockedPlaceholderTemplate, labels[0], labels[1])
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

func Sanitize(htmlSrc string, opts Options) (string, error) {
	rewritten, err := rewriteImages(htmlSrc, opts)
	if err != nil {
		return "", err
	}
	return policy().Sanitize(rewritten), nil
}

func policy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowRelativeURLs(true)
	p.AllowURLSchemes("http", "https", "mailto", "data")
	p.AllowAttrs("style").Globally()
	p.AllowAttrs("class").Globally()
	p.AllowAttrs("data-remote-blocked", "data-original-src").OnElements("img")
	return p
}

func rewriteImages(htmlSrc string, opts Options) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlSrc))
	if err != nil {
		return "", err
	}
	walk(doc, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "img" {
			return
		}
		for i, a := range n.Attr {
			if a.Key != "src" {
				continue
			}
			newSrc, blocked, original := rewriteSrc(a.Val, opts)
			n.Attr[i].Val = newSrc
			if blocked {
				n.Attr = append(n.Attr,
					html.Attribute{Key: "data-remote-blocked", Val: "1"},
					html.Attribute{Key: "data-original-src", Val: original},
				)
			}
			break
		}
	})
	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func walk(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

func rewriteSrc(src string, opts Options) (newSrc string, blocked bool, original string) {
	placeholderSrc := blockedPlaceholder(opts.Lang)
	trimmed := strings.TrimSpace(src)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "cid:") {
		cid := trimmed[len("cid:"):]
		return opts.CIDBaseURL + url.PathEscape(cid), false, ""
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return placeholderSrc, true, trimmed
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		if !opts.ShowRemote {
			return placeholderSrc, true, trimmed
		}
		return trimmed, false, ""
	}
	return trimmed, false, ""
}
