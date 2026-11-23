package store

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestParseSearch(t *testing.T) {
	cases := []struct {
		in   string
		want []searchClause
	}{
		{"", nil},
		{"  hello  world ", []searchClause{{value: "hello"}, {value: "world"}}},
		{`"exact phrase" -spam`, []searchClause{
			{value: "exact phrase", phrase: true}, {value: "spam", negate: true}}},
		{`From:alice subject:"q3 report" -has:attachment`, []searchClause{
			{field: "from", value: "alice"},
			{field: "subject", value: "q3 report", phrase: true},
			{field: "has", value: "attachment", negate: true}}},
		// Unknown keys and colons in ordinary text stay plain terms.
		{"http://x.com 10:30 foo:bar", []searchClause{
			{value: "http://x.com"}, {value: "10:30"}, {value: "foo:bar"}}},
		// Half-typed operators and a lone dash are dropped or literal.
		{"from: -", []searchClause{{value: "-"}}},
		{`subject:"unterminated`, []searchClause{{field: "subject", value: "unterminated", phrase: true}}},
	}
	for _, c := range cases {
		got := parseSearch(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseSearch(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestParsePeriodAndSize(t *testing.T) {
	utc := time.UTC
	unix := func(y int, m time.Month, d int) int64 { return time.Date(y, m, d, 0, 0, 0, 0, utc).Unix() }
	periods := []struct {
		in         string
		start, end int64
		ok         bool
	}{
		{"2019", unix(2019, 1, 1), unix(2020, 1, 1), true},
		{"2019-03", unix(2019, 3, 1), unix(2019, 4, 1), true},
		{"2019/03/05", unix(2019, 3, 5), unix(2019, 3, 6), true},
		{"2019.12.31", unix(2019, 12, 31), unix(2020, 1, 1), true},
		{"2019-02-31", 0, 0, false},
		{"2019-13", 0, 0, false},
		{"yesterday", 0, 0, false},
	}
	for _, p := range periods {
		s, e, ok := parsePeriod(p.in, utc)
		if ok != p.ok || s != p.start || e != p.end {
			t.Errorf("parsePeriod(%q) = %d,%d,%v want %d,%d,%v", p.in, s, e, ok, p.start, p.end, p.ok)
		}
	}
	sizes := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"500", 500, true}, {"500K", 500 << 10, true}, {"2mb", 2 << 20, true},
		{"1.5M", 3 << 19, true}, {"1G", 1 << 30, true}, {"big", 0, false},
	}
	for _, c := range sizes {
		got, ok := parseSize(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseSize(%q) = %d,%v want %d,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestListEmailsSearchOperators(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	at := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 12, 0, 0, 0, time.Local) }

	rows := []EmailRow{
		{Email: Email{SHA256: "receipt", Subject: "Your order receipt", FromAddr: "Shop <orders@shop.example>",
			ToAddrs: []string{"me@home.example"}, SentAt: at(2019, 3, 5), SizeBytes: 80 << 10,
			HasAttachments: true, AttachmentCount: 1}, BodyText: "Thanks for buying a kettle"},
		{Email: Email{SHA256: "report", Subject: "Q3 report", FromAddr: "Alice <alice@work.example>",
			ToAddrs: []string{"team@work.example"}, CcAddrs: []string{"me@home.example"},
			SentAt: at(2021, 10, 1), SizeBytes: 3 << 20, HasAttachments: true, AttachmentCount: 2},
			BodyText: "Numbers attached"},
		{Email: Email{SHA256: "hello", Subject: "한국 소식", FromAddr: "Bob <bob@home.example>",
			ToAddrs: []string{"me@home.example"}, SentAt: at(2021, 10, 20), SizeBytes: 2 << 10},
			BodyText: "just saying hi about the report"},
	}
	ids := map[string]int64{}
	for _, r := range rows {
		id, err := s.InsertEmail(ctx, r)
		if err != nil {
			t.Fatal(err)
		}
		ids[r.SHA256] = id
	}
	// Attachment rows are written by the attachment indexer; insert directly.
	for _, a := range []struct {
		sha  string
		idx  int
		name string
	}{
		{"receipt", 0, "invoice_2019-03.pdf"},
		{"report", 0, "q3-numbers.xlsx"},
		{"report", 1, "chart 100%.png"},
	} {
		if _, err := s.DB.ExecContext(ctx,
			`INSERT INTO attachments (email_id, idx, filename) VALUES (?,?,?)`,
			ids[a.sha], a.idx, a.name); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetEmailStarred(ctx, "report", true); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		q    string
		want []string
	}{
		{"report", []string{"hello", "report"}},
		{"kettle", []string{"receipt"}},
		{"invoice", []string{"receipt"}}, // attachment name, plain word
		{"subject:report", []string{"report"}},
		{"-subject:report report", []string{"hello"}},
		{"from:alice", []string{"report"}},
		{"from:shop.example", []string{"receipt"}},
		{"to:team", []string{"report"}},
		{"to:me@home.example", []string{"hello", "receipt", "report"}}, // Cc counts
		{"cc:me@home", []string{"report"}},
		{"filename:pdf", []string{"receipt"}},
		{"filename:100%", []string{"report"}},
		{"filename:3_n", []string{}}, // LIKE wildcards are literal (3-n exists)
		{"has:attachment", []string{"receipt", "report"}},
		{"-has:attachment", []string{"hello"}},
		{"is:starred", []string{"report"}},
		{"-is:starred", []string{"hello", "receipt"}},
		{"is:unread", []string{"hello", "receipt", "report"}},
		{"after:2021", []string{"hello", "report"}},
		{"before:2021", []string{"receipt"}},
		{"after:2021-10-02 before:2021-11", []string{"hello"}},
		{"on:2019-03", []string{"receipt"}},
		{"on:2019-03-06", []string{}},
		{"larger:1M", []string{"report"}},
		{"smaller:10K", []string{"hello"}},
		{`"order receipt"`, []string{"receipt"}},
		{`"receipt order"`, []string{}},
		{"ㅎㄱ", []string{"hello"}},
		{"ㅎㄱ from:bob", []string{"hello"}},
		{"from:", []string{"hello", "receipt", "report"}}, // half-typed: no filter
		{"after:someday", []string{"hello", "receipt", "report"}},
	}
	for _, c := range cases {
		got, total, err := s.ListEmails(ctx, ListOptions{Query: c.q, Limit: 50})
		if err != nil {
			t.Fatalf("%q: %v", c.q, err)
		}
		shas := []string{}
		for _, e := range got {
			shas = append(shas, e.SHA256)
		}
		sort.Strings(shas)
		if !reflect.DeepEqual(shas, c.want) || total != len(c.want) {
			t.Errorf("%q = %v (total %d), want %v", c.q, shas, total, c.want)
		}
	}
}
