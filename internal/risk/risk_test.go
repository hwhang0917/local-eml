package risk

import (
	"strings"
	"testing"

	"github.com/jhillyerd/enmime"
)

func msg(t *testing.T, headers, html string) *enmime.Envelope {
	t.Helper()
	raw := headers + "\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=utf-8\r\n\r\n" + html
	env, err := enmime.ReadEnvelope(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func codes(rs []Reason) string {
	var s []string
	for _, r := range rs {
		s = append(s, r.Code)
	}
	return strings.Join(s, ",")
}

func TestCleanMailHasNoReasons(t *testing.T) {
	// A normal newsletter: tracking links behind prose, a bare domain that
	// matches its target, subdomain vs apex, a mailto and a relative link.
	env := msg(t, "From: Shop <news@shop.example>\r\nReply-To: help@shop.example\r\nSubject: hi",
		`<p><a href="https://click.shop.example/t/abc">Read more</a>
		 <a href="https://www.shop.example/sale">shop.example</a>
		 <a href="https://shop.example/x">https://shop.example/y</a>
		 <a href="mailto:help@shop.example">help@shop.example</a>
		 <a href="/local">example.com</a></p>`)
	if rs := Assess(env); len(rs) != 0 {
		t.Fatalf("clean mail flagged: %+v", rs)
	}
}

func TestSignals(t *testing.T) {
	cases := []struct {
		name, headers, html, want string
	}{
		{"link text lies about destination",
			"From: a@bank.example", `<a href="https://login.evil.example/x">https://bank.example/login</a>`, LinkMismatch},
		{"link to bare IP",
			"From: a@bank.example", `<a href="http://203.0.113.9/login">Sign in</a>`, IPLink},
		{"punycode link host",
			"From: a@bank.example", `<a href="https://xn--pypal-4ve.example/">Sign in</a>`, IDNDomain},
		{"non-ascii sender domain",
			"From: a@pаypal.example", ``, IDNDomain}, // Cyrillic а
		{"dmarc fail stamped by receiver",
			"From: a@bank.example\r\nAuthentication-Results: mx.example; spf=pass; dkim=fail; dmarc=fail (p=reject)", ``, AuthFail + "," + AuthFail},
		{"reply-to on another site",
			"From: a@bank.example\r\nReply-To: collect@free-mail.example", ``, ReplyToMismatch},
		{"display name embeds a different address",
			`From: "support@bank.example" <x@evil.example>`, ``, SenderMismatch},
		{"password field",
			"From: a@bank.example", `<form action="https://evil.example/p"><input type="password"></form>`, CredentialForm + "," + CredentialForm},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := codes(Assess(msg(t, c.headers, c.html))); got != c.want {
				t.Fatalf("codes = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDangerousAttachment(t *testing.T) {
	raw := "From: a@x.example\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=b\r\n\r\n" +
		"--b\r\nContent-Type: text/plain\r\n\r\nsee attached\r\n" +
		"--b\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=\"invoice.pdf.exe\"\r\n\r\nMZ\r\n" +
		"--b\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename=\"real.pdf\"\r\n\r\n%PDF\r\n--b--\r\n"
	env, err := enmime.ReadEnvelope(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	rs := Assess(env)
	if codes(rs) != DangerousAttachment || rs[0].Detail != "invoice.pdf.exe" {
		t.Fatalf("got %+v", rs)
	}
}

func TestReasonsAreCappedAndDeduped(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString(`<a href="http://198.51.100.7/">x</a>`) // same detail 50 times
	}
	for i := 0; i < 50; i++ {
		b.WriteString(`<a href="http://198.51.100.` + string(rune('0'+i%10)) + `0/">y</a>`)
	}
	rs := Assess(msg(t, "From: a@x.example", b.String()))
	if len(rs) == 0 || len(rs) > maxReasons {
		t.Fatalf("len = %d, want 1..%d", len(rs), maxReasons)
	}
}
