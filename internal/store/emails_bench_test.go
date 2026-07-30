package store

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"
)

// Catalog size for BenchmarkListEmails. Defaults to 10k so a casual
// `go test -bench` stays fast; export LOCAL_EML_BENCH_N=100000 to reproduce
// the large-catalog numbers quoted in the README.
const benchSizeEnv = "LOCAL_EML_BENCH_N"

var benchVocab = []string{
	"invoice", "meeting", "quarterly", "planning", "deploy", "release", "budget",
	"schedule", "review", "newsletter", "security", "alert", "shipment", "order",
	"confirmation", "receipt", "project", "update", "weekly", "report", "server",
	"database", "migration", "holiday", "party", "contract", "renewal", "password",
	"reset", "welcome", "onboarding", "feedback", "survey", "결제", "청구서", "회의",
	"일정", "배송", "주문", "확인", "보고서", "서버", "점검", "안내", "이벤트", "할인",
}

func benchWords(r *rand.Rand, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			out += " "
		}
		out += benchVocab[r.Intn(len(benchVocab))]
	}
	return out
}

func benchCatalog(b *testing.B, n int) *Store {
	b.Helper()
	ctx := context.Background()
	st, err := Open(ctx, b.TempDir()+"/bench.db")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { st.Close() })
	// Fixture-only: durability is irrelevant for a throwaway DB and fsyncs
	// dominate 100k inserts.
	st.DB.Exec("PRAGMA synchronous=OFF")

	r := rand.New(rand.NewSource(42))
	for i := 0; i < n; i++ {
		row := EmailRow{
			Email: Email{
				SHA256:    fmt.Sprintf("%064x", i),
				Filename:  fmt.Sprintf("msg%06d.eml", i),
				Subject:   benchWords(r, 5),
				FromAddr:  fmt.Sprintf("sender%d@example%d.com", r.Intn(500), r.Intn(50)),
				ToAddrs:   []string{"me@example.com"},
				SentAt:    time.Unix(1500000000+int64(r.Intn(300000000)), 0),
				SizeBytes: int64(2000 + r.Intn(50000)),
			},
			BodyText: benchWords(r, 300),
		}
		// ~30% of mail belongs to a conversation of ~3 messages.
		if r.Intn(10) < 3 {
			row.ThreadID = fmt.Sprintf("thr-%d@example.com", i/3)
		}
		if _, err := st.InsertEmail(ctx, row); err != nil {
			b.Fatal(err)
		}
	}
	// A rare term with ~50 hits, so there is a selective query to measure —
	// every word in benchVocab matches nearly the whole catalog.
	for i := 0; i < 50; i++ {
		if _, err := st.InsertEmail(ctx, EmailRow{
			Email: Email{
				SHA256:   fmt.Sprintf("%056xzebra%03d", i, i),
				Filename: fmt.Sprintf("zebra%03d.eml", i),
				Subject:  "zebra sighting " + benchWords(r, 3),
				FromAddr: "zoo@example.com",
				ToAddrs:  []string{"me@example.com"},
				SentAt:   time.Unix(1600000000+int64(i), 0),
			},
			BodyText: "zebra " + benchWords(r, 200),
		}); err != nil {
			b.Fatal(err)
		}
	}
	return st
}

func BenchmarkListEmails(b *testing.B) {
	n := 10_000
	if v := os.Getenv(benchSizeEnv); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed <= 0 {
			b.Fatalf("bad %s: %q", benchSizeEnv, v)
		}
		n = parsed
	}
	st := benchCatalog(b, n)
	ctx := context.Background()

	cases := []struct {
		name string
		opts ListOptions
	}{
		{"flat", ListOptions{}},
		{"fts_selective", ListOptions{Query: "zebra"}},
		{"fts_common_word", ListOptions{Query: "invoice"}},
		{"fts_korean", ListOptions{Query: "청구서"}},
		{"chosung_like", ListOptions{Query: "ㅊㄱㅅ"}},
		{"grouped", ListOptions{GroupThreads: true}},
		{"grouped_fts_common", ListOptions{Query: "invoice", GroupThreads: true}},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			c.opts.Limit = 50
			for i := 0; i < b.N; i++ {
				if _, _, err := st.ListEmails(ctx, c.opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
