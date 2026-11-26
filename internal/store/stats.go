package store

import (
	"context"
	"fmt"
	"net/mail"
	"sort"
	"strings"
)

const (
	topSendersLimit = 10
	largestLimit    = 5
	// maxTZOffsetMinutes bounds the caller's UTC offset (UTC-12 to UTC+14).
	maxTZOffsetMinutes = 14 * 60
)

type YearCount struct {
	Year  string `json:"year"`
	Count int    `json:"count"`
}

// MonthCount is one calendar month ("2024-06") in the caller's time zone.
type MonthCount struct {
	Month string `json:"month"`
	Count int    `json:"count"`
	Bytes int64  `json:"bytes"`
}

// SenderCount groups every From header that carries the same address, so
// "Alice <a@x.com>" and "a@x.com" count as one sender. Name is the display
// name seen most often for that address.
type SenderCount struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	Count   int    `json:"count"`
}

type DomainCount struct {
	Domain string `json:"domain"`
	Count  int    `json:"count"`
}

type CategoryCount struct {
	CategoryID int64 `json:"category_id"`
	Count      int   `json:"count"`
}

// KindUsage is the attachments of one kind (see AttachmentKind). Bytes is the
// decoded size; inside the .eml the same file takes about a third more as
// base64.
type KindUsage struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
	Bytes int64  `json:"bytes"`
}

type LargeEmail struct {
	SHA256    string `json:"sha256"`
	Subject   string `json:"subject"`
	From      string `json:"from"`
	SentAt    int64  `json:"sent_at"`
	SizeBytes int64  `json:"size_bytes"`
}

type Stats struct {
	TotalCount         int   `json:"total_count"`
	TotalBytes         int64 `json:"total_bytes"`
	StarredCount       int   `json:"starred_count"`
	AttachmentCount    int   `json:"attachment_count"`
	UndatedCount       int   `json:"undated_count"`
	UnreadCount        int   `json:"unread_count"`
	SpamCount          int   `json:"spam_count"`
	PhishingCount      int   `json:"phishing_count"`
	RiskyCount         int   `json:"risky_count"`
	UncategorizedCount int   `json:"uncategorized_count"`

	PerYear  []YearCount  `json:"per_year"`
	PerMonth []MonthCount `json:"per_month"`
	// Heatmap[weekday][hour] counts dated mail by when it was sent, in the
	// caller's time zone. Weekday 0 is Sunday.
	Heatmap     [7][24]int      `json:"heatmap"`
	TopSenders  []SenderCount   `json:"top_senders"`
	TopDomains  []DomainCount   `json:"top_domains"`
	PerCategory []CategoryCount `json:"per_category"`

	AttachmentBytes int64        `json:"attachment_bytes"`
	PerKind         []KindUsage  `json:"per_kind"`
	Largest         []LargeEmail `json:"largest"`
}

// Stats aggregates over the emails and attachments tables only; it never
// touches blobs. tzOffsetMinutes is the caller's offset east of UTC (540 for
// Seoul), used to bucket months, weekdays and hours the way the user sees
// them. It is one fixed offset, so mail from the other side of a DST change
// lands an hour off; for a whole-archive heatmap that blur doesn't matter.
func (s *Store) Stats(ctx context.Context, tzOffsetMinutes int) (*Stats, error) {
	if tzOffsetMinutes > maxTZOffsetMinutes || tzOffsetMinutes < -maxTZOffsetMinutes {
		tzOffsetMinutes = 0
	}
	// An SQLite date modifier that moves a unix timestamp into the caller's zone.
	shift := fmt.Sprintf("%+d minutes", tzOffsetMinutes)

	st := &Stats{}
	if err := s.statsTotals(ctx, st); err != nil {
		return nil, err
	}
	var err error
	if st.PerYear, err = s.statsPerYear(ctx, shift); err != nil {
		return nil, err
	}
	if st.PerMonth, err = s.statsPerMonth(ctx, shift); err != nil {
		return nil, err
	}
	if st.Heatmap, err = s.statsHeatmap(ctx, shift); err != nil {
		return nil, err
	}
	if st.TopSenders, st.TopDomains, err = s.statsTopSenders(ctx); err != nil {
		return nil, err
	}
	if st.PerCategory, err = s.statsPerCategory(ctx); err != nil {
		return nil, err
	}
	if st.PerKind, st.AttachmentBytes, err = s.statsPerKind(ctx); err != nil {
		return nil, err
	}
	if st.Largest, err = s.statsLargest(ctx); err != nil {
		return nil, err
	}
	return st, nil
}

func (s *Store) statsTotals(ctx context.Context, st *Stats) error {
	return s.DB.QueryRowContext(ctx, `
		SELECT COUNT(*), IFNULL(SUM(size_bytes), 0), IFNULL(SUM(starred), 0),
			IFNULL(SUM(has_attachments), 0), IFNULL(SUM(sent_at = 0), 0),
			IFNULL(SUM(read = 0), 0), IFNULL(SUM(flag = ?), 0),
			IFNULL(SUM(flag = ?), 0),
			IFNULL(SUM(risk IS NOT NULL AND risk NOT IN ('', '[]', 'null')), 0),
			IFNULL(SUM(category_id IS NULL), 0)
		FROM emails`, FlagSpam, FlagPhishing).Scan(
		&st.TotalCount, &st.TotalBytes, &st.StarredCount,
		&st.AttachmentCount, &st.UndatedCount,
		&st.UnreadCount, &st.SpamCount, &st.PhishingCount, &st.RiskyCount,
		&st.UncategorizedCount,
	)
}

func (s *Store) statsPerYear(ctx context.Context, shift string) ([]YearCount, error) {
	return queryAll(ctx, s.DB, func(rs rowScanner) (YearCount, error) {
		var yc YearCount
		err := rs.Scan(&yc.Year, &yc.Count)
		return yc, err
	}, `
		SELECT strftime('%Y', sent_at, 'unixepoch', ?), COUNT(*)
		FROM emails WHERE sent_at > 0 GROUP BY 1 ORDER BY 1`, shift)
}

func (s *Store) statsPerMonth(ctx context.Context, shift string) ([]MonthCount, error) {
	return queryAll(ctx, s.DB, func(rs rowScanner) (MonthCount, error) {
		var mc MonthCount
		err := rs.Scan(&mc.Month, &mc.Count, &mc.Bytes)
		return mc, err
	}, `
		SELECT strftime('%Y-%m', sent_at, 'unixepoch', ?), COUNT(*), IFNULL(SUM(size_bytes), 0)
		FROM emails WHERE sent_at > 0 GROUP BY 1 ORDER BY 1`, shift)
}

func (s *Store) statsHeatmap(ctx context.Context, shift string) ([7][24]int, error) {
	type cell struct{ weekday, hour, count int }
	var heatmap [7][24]int
	cells, err := queryAll(ctx, s.DB, func(rs rowScanner) (cell, error) {
		var c cell
		err := rs.Scan(&c.weekday, &c.hour, &c.count)
		return c, err
	}, `
		SELECT CAST(strftime('%w', sent_at, 'unixepoch', ?1) AS INTEGER),
			CAST(strftime('%H', sent_at, 'unixepoch', ?1) AS INTEGER), COUNT(*)
		FROM emails WHERE sent_at > 0 GROUP BY 1, 2`, shift)
	if err != nil {
		return heatmap, err
	}
	for _, c := range cells {
		if c.weekday >= 0 && c.weekday < 7 && c.hour >= 0 && c.hour < 24 {
			heatmap[c.weekday][c.hour] = c.count
		}
	}
	return heatmap, nil
}

func (s *Store) statsTopSenders(ctx context.Context) ([]SenderCount, []DomainCount, error) {
	raw, err := queryAll(ctx, s.DB, func(rs rowScanner) (SenderCount, error) {
		var sc SenderCount
		err := rs.Scan(&sc.Name, &sc.Count)
		return sc, err
	}, `SELECT from_addr, COUNT(*) FROM emails WHERE from_addr != '' GROUP BY from_addr`)
	if err != nil {
		return nil, nil, err
	}
	senders, domains := groupSenders(raw)
	return senders, domains, nil
}

func (s *Store) statsPerCategory(ctx context.Context) ([]CategoryCount, error) {
	return queryAll(ctx, s.DB, func(rs rowScanner) (CategoryCount, error) {
		var cc CategoryCount
		err := rs.Scan(&cc.CategoryID, &cc.Count)
		return cc, err
	}, `SELECT category_id, COUNT(*) FROM emails WHERE category_id IS NOT NULL GROUP BY category_id`)
}

// statsPerKind lists attachment usage by kind, largest first, along with the
// total attachment bytes.
func (s *Store) statsPerKind(ctx context.Context) ([]KindUsage, int64, error) {
	usage, err := queryAll(ctx, s.DB, func(rs rowScanner) (KindUsage, error) {
		var k KindUsage
		err := rs.Scan(&k.Kind, &k.Count, &k.Bytes)
		return k, err
	}, `SELECT `+kindExpr+`, COUNT(*), IFNULL(SUM(a.size_bytes), 0) FROM attachments a GROUP BY 1`)
	if err != nil {
		return nil, 0, err
	}
	byKind := map[string]KindUsage{}
	var totalBytes int64
	for _, k := range usage {
		byKind[k.Kind] = k
		totalBytes += k.Bytes
	}
	// Walk the fixed kind order first so equal sizes keep a stable order.
	perKind := []KindUsage{}
	for _, kind := range AttachmentKinds {
		if k, ok := byKind[kind]; ok {
			perKind = append(perKind, k)
		}
	}
	sort.SliceStable(perKind, func(i, j int) bool { return perKind[i].Bytes > perKind[j].Bytes })
	return perKind, totalBytes, nil
}

func (s *Store) statsLargest(ctx context.Context) ([]LargeEmail, error) {
	return queryAll(ctx, s.DB, func(rs rowScanner) (LargeEmail, error) {
		var e LargeEmail
		err := rs.Scan(&e.SHA256, &e.Subject, &e.From, &e.SentAt, &e.SizeBytes)
		return e, err
	}, `
		SELECT sha256, IFNULL(subject, ''), IFNULL(from_addr, ''), IFNULL(sent_at, 0), IFNULL(size_bytes, 0)
		FROM emails ORDER BY size_bytes DESC, id LIMIT ?`, largestLimit)
}

// groupSenders folds raw From headers (Name holds the header, Count its
// messages) into the top senders by address and the top sending domains.
func groupSenders(raw []SenderCount) ([]SenderCount, []DomainCount) {
	type agg struct {
		count     int
		name      string
		nameCount int
	}
	byAddr := map[string]*agg{}
	byDomain := map[string]int{}
	for _, r := range raw {
		name, addr := splitFrom(r.Name)
		a := byAddr[addr]
		if a == nil {
			a = &agg{}
			byAddr[addr] = a
		}
		a.count += r.Count
		if name != "" && r.Count > a.nameCount {
			a.name, a.nameCount = name, r.Count
		}
		if i := strings.LastIndexByte(addr, '@'); i >= 0 && i < len(addr)-1 {
			byDomain[addr[i+1:]] += r.Count
		}
	}

	senders := make([]SenderCount, 0, len(byAddr))
	for addr, a := range byAddr {
		senders = append(senders, SenderCount{Address: addr, Name: a.name, Count: a.count})
	}
	sort.Slice(senders, func(i, j int) bool {
		if senders[i].Count != senders[j].Count {
			return senders[i].Count > senders[j].Count
		}
		return senders[i].Address < senders[j].Address
	})
	if len(senders) > topSendersLimit {
		senders = senders[:topSendersLimit]
	}

	domains := make([]DomainCount, 0, len(byDomain))
	for d, c := range byDomain {
		domains = append(domains, DomainCount{Domain: d, Count: c})
	}
	sort.Slice(domains, func(i, j int) bool {
		if domains[i].Count != domains[j].Count {
			return domains[i].Count > domains[j].Count
		}
		return domains[i].Domain < domains[j].Domain
	})
	if len(domains) > topSendersLimit {
		domains = domains[:topSendersLimit]
	}
	return senders, domains
}

// splitFrom pulls the display name and lower-cased address out of a From
// header. Headers net/mail rejects fall back to the text between < and >, or
// the whole header.
func splitFrom(h string) (name, addr string) {
	if a, err := mail.ParseAddress(h); err == nil {
		return a.Name, strings.ToLower(a.Address)
	}
	h = strings.TrimSpace(h)
	if i := strings.LastIndexByte(h, '<'); i >= 0 {
		if j := strings.IndexByte(h[i:], '>'); j > 0 {
			return strings.Trim(strings.TrimSpace(h[:i]), `"`), strings.ToLower(strings.TrimSpace(h[i+1 : i+j]))
		}
	}
	return "", strings.ToLower(h)
}
