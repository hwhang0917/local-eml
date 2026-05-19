package sanitize

import (
	"bytes"
	"net/url"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

type Options struct {
	CIDBaseURL string
	ShowRemote bool
}

const placeholderSrc = "data:image/gif;base64,R0lGODlhAQABAIAAAP///wAAACH5BAEAAAAALAAAAAABAAEAAAICRAEAOw=="

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
