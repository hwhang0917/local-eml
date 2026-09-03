// Package risk spots structural signs of phishing in a parsed message. It is
// a warning, never a verdict: every signal is language-neutral, computed
// offline, and chosen for precision over recall — a false positive costs the
// user a glance at a banner, so keyword heuristics are deliberately absent.
package risk

import (
	"net"
	"net/mail"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/jhillyerd/enmime"
	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"
)

// Reason is one finding. Code is stable (the UI maps it to a label); Detail
// is the offending value, shown verbatim.
type Reason struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

const (
	LinkMismatch        = "link_mismatch"        // anchor text names one site, href goes to another
	IPLink              = "ip_link"              // link to a bare IP address
	IDNDomain           = "idn_domain"           // punycode / non-ASCII host (homoglyph lookalikes)
	AuthFail            = "auth_fail"            // Authentication-Results: spf/dkim/dmarc=fail
	ReplyToMismatch     = "reply_to_mismatch"    // Reply-To on a different site than From
	SenderMismatch      = "sender_mismatch"      // display name embeds an address on another site
	DangerousAttachment = "dangerous_attachment" // executable or script attachment
	CredentialForm      = "credential_form"      // HTML form or password field in the body
)

// ponytail: cap keeps the stored JSON small on link-farm spam.
const maxReasons = 12

var (
	authFailRE  = regexp.MustCompile(`\b(spf|dkim|dmarc)=fail\b`)
	emailRE     = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	hostTextRE  = regexp.MustCompile(`(?i)^(?:https?://)?([a-z0-9-]+(?:\.[a-z0-9-]+)+)(?:[/:?#]\S*)?$`)
	dangerousXT = map[string]bool{
		".exe": true, ".scr": true, ".com": true, ".pif": true, ".msi": true, ".jar": true,
		".js": true, ".jse": true, ".vbs": true, ".vbe": true, ".wsf": true, ".hta": true,
		".cmd": true, ".bat": true, ".ps1": true, ".reg": true, ".lnk": true, ".iso": true, ".img": true,
		".htm": true, ".html": true, ".docm": true, ".xlsm": true, ".pptm": true,
	}
)

// Assess returns the findings for env, empty when nothing looks off.
func Assess(env *enmime.Envelope) []Reason {
	out := []Reason{}
	seen := map[string]bool{}
	add := func(code, detail string) {
		if k := code + "\x00" + detail; !seen[k] && len(out) < maxReasons {
			seen[k] = true
			out = append(out, Reason{Code: code, Detail: detail})
		}
	}

	fromAddr, fromName := parseAddr(env.GetHeader("From"))
	fromHost := hostOf(fromAddr)
	if embedded := emailRE.FindString(fromName); embedded != "" && !sameSite(hostOf(embedded), fromHost) {
		add(SenderMismatch, fromName+" <"+fromAddr+">")
	}
	if isIDN(fromHost) {
		add(IDNDomain, fromHost)
	}
	if rt, _ := parseAddr(env.GetHeader("Reply-To")); rt != "" && fromHost != "" {
		if h := hostOf(rt); h != "" && !sameSite(h, fromHost) {
			add(ReplyToMismatch, rt)
		}
	}
	for _, v := range env.GetHeaderValues("Authentication-Results") {
		for _, m := range authFailRE.FindAllString(strings.ToLower(v), -1) {
			add(AuthFail, m)
		}
	}
	scanHTML(env.HTML, add)
	for _, p := range env.Attachments {
		if dangerousXT[strings.ToLower(path.Ext(p.FileName))] {
			add(DangerousAttachment, p.FileName)
		}
	}
	return out
}

func scanHTML(src string, add func(code, detail string)) {
	if src == "" {
		return
	}
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "a":
				checkLink(attr(n, "href"), strings.TrimSpace(textOf(n)), add)
			case "form":
				add(CredentialForm, "<form>")
			case "input":
				if strings.EqualFold(attr(n, "type"), "password") {
					add(CredentialForm, "password field")
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
}

func checkLink(href, text string, add func(code, detail string)) {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return
	}
	if net.ParseIP(host) != nil {
		add(IPLink, host)
	} else if isIDN(host) {
		add(IDNDomain, host)
	}
	// Only anchor text that itself names a site can lie about where it goes;
	// "Read more" over a tracking link is how every newsletter works.
	if m := hostTextRE.FindStringSubmatch(text); m != nil {
		if th := strings.ToLower(m[1]); registrable(th) != "" && !sameSite(th, host) {
			add(LinkMismatch, text+" → "+host)
		}
	}
}

// sameSite compares registrable domains (eTLD+1), so mail.example.com and
// www.example.com agree while example.com and example.co do not.
func sameSite(a, b string) bool {
	ra, rb := registrable(a), registrable(b)
	if ra == "" || rb == "" {
		return strings.TrimPrefix(a, "www.") == strings.TrimPrefix(b, "www.")
	}
	return ra == rb
}

func registrable(host string) string {
	d, err := publicsuffix.EffectiveTLDPlusOne(strings.ToLower(strings.TrimSuffix(host, ".")))
	if err != nil {
		return ""
	}
	return d
}

func isIDN(host string) bool {
	for _, label := range strings.Split(host, ".") {
		if strings.HasPrefix(label, "xn--") {
			return true
		}
	}
	for _, r := range host {
		if r > 0x7f {
			return true
		}
	}
	return false
}

func hostOf(addr string) string {
	if i := strings.LastIndex(addr, "@"); i >= 0 {
		return strings.ToLower(addr[i+1:])
	}
	return ""
}

// parseAddr is lenient: phishing mail is exactly where headers are malformed.
func parseAddr(h string) (addr, name string) {
	if h == "" {
		return "", ""
	}
	if a, err := mail.ParseAddress(h); err == nil {
		return strings.ToLower(a.Address), a.Name
	}
	if m := emailRE.FindString(h); m != "" {
		name := strings.TrimSpace(strings.Replace(h, m, "", 1))
		return strings.ToLower(m), strings.Trim(name, `<>" `)
	}
	return "", h
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}
