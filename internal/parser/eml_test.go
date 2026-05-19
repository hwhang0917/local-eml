package parser

import (
	"strings"
	"testing"
)

const sampleEML = "From: alice@example.com\r\n" +
	"To: bob@example.com\r\n" +
	"Cc: carol@example.com\r\n" +
	"Subject: Test Subject\r\n" +
	"Date: Mon, 02 Jan 2006 15:04:05 -0700\r\n" +
	"Message-ID: <test@example.com>\r\n" +
	"MIME-Version: 1.0\r\n" +
	"Content-Type: text/plain; charset=utf-8\r\n" +
	"\r\n" +
	"Hello, this is the body.\r\n"

func TestParseExtractsMetadata(t *testing.T) {
	p, err := Parse(strings.NewReader(sampleEML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p.Subject != "Test Subject" {
		t.Errorf("Subject = %q", p.Subject)
	}
	if !strings.Contains(p.From, "alice@example.com") {
		t.Errorf("From = %q", p.From)
	}
	if len(p.To) != 1 || !strings.Contains(p.To[0], "bob@example.com") {
		t.Errorf("To = %v", p.To)
	}
	if p.MessageID != "<test@example.com>" {
		t.Errorf("MessageID = %q", p.MessageID)
	}
	if !strings.Contains(p.BodyText, "Hello") {
		t.Errorf("BodyText missing greeting: %q", p.BodyText)
	}
	if p.Date.Year() != 2006 {
		t.Errorf("Date = %v", p.Date)
	}
}

const sampleHTMLEML = "From: a@b.com\r\n" +
	"To: c@d.com\r\n" +
	"Subject: HTML\r\n" +
	"MIME-Version: 1.0\r\n" +
	"Content-Type: text/html; charset=utf-8\r\n" +
	"\r\n" +
	"<p>Hello <b>world</b> &amp; friends</p>\r\n"

func TestParseHTMLOnlyFallback(t *testing.T) {
	p, err := Parse(strings.NewReader(sampleHTMLEML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !p.HTMLAvailable {
		t.Error("HTMLAvailable should be true")
	}
	if !strings.Contains(p.BodyText, "Hello") || !strings.Contains(p.BodyText, "world") {
		t.Errorf("html-to-text fallback failed: %q", p.BodyText)
	}
	if !strings.Contains(p.BodyText, "&") {
		t.Errorf("entity not decoded: %q", p.BodyText)
	}
}

func TestHTMLToTextStripsTags(t *testing.T) {
	got := htmlToText("<p>Hi  <strong>there</strong></p>")
	want := "Hi there"
	if got != want {
		t.Errorf("htmlToText = %q, want %q", got, want)
	}
}

func TestSplitAddrs(t *testing.T) {
	got := splitAddrs("a@x, b@y , c@z")
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	if got[1] != "b@y" {
		t.Errorf("got[1] = %q", got[1])
	}
}
