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
	shift := fmt.Sprintf("%+d minutes", tzOffsetMinutes)

	st := &Stats{
		PerYear:     []YearCount{},
		PerMonth:    []MonthCount{},
		TopSenders:  []SenderCount{},
		TopDomains:  []DomainCount{},
		PerCategory: []CategoryCount{},
		PerKind:     []KindUsage{},
		Largest:     []LargeEmail{},
	}

	if err := s.DB.QueryRowContext(ctx, `
		SELECT COUNT(*), IFNULL(SUM(size_bytes), 0), IFNULL(SUM(starred), 0),
			IFNULL(SUM(has_attachments), 0), IFNULL(SUM(sent_at = 0), 0),
			IFNULL(SUM(read = 0), 0), IFNULL(SUM(flag = 'spam'), 0),
			IFNULL(SUM(flag = 'phishing'), 0),
			IFNULL(SUM(risk IS NOT NULL AND risk NOT IN ('', '[]', 'null')), 0),
			IFNULL(SUM(category_id IS NULL), 0)
		FROM emails`).Scan(
		&st.TotalCount, &st.TotalBytes, &st.StarredCount,
		&st.AttachmentCount, &st.UndatedCount,
		&st.UnreadCount, &st.SpamCount, &st.PhishingCount, &st.RiskyCount,
		&st.UncategorizedCount,
	); err != nil {
		return nil, err
	}

	if err := s.eachRow(ctx, func(scan func(...any) error) error {
		var yc YearCount
		if err := scan(&yc.Year, &yc.Count); err != nil {
			return err
		}
		st.PerYear = append(st.PerYear, yc)
		return nil
	}, `
		SELECT strftime('%Y', sent_at, 'unixepoch', ?), COUNT(*)
		FROM emails WHERE sent_at > 0 GROUP BY 1 ORDER BY 1`, shift); err != nil {
		return nil, err
	}

	if err := s.eachRow(ctx, func(scan func(...any) error) error {
		var mc MonthCount
		if err := scan(&mc.Month, &mc.Count, &mc.Bytes); err != nil {
			return err
		}
		st.PerMonth = append(st.PerMonth, mc)
		return nil
	}, `
		SELECT strftime('%Y-%m', sent_at, 'unixepoch', ?), COUNT(*), IFNULL(SUM(size_bytes), 0)
		FROM emails WHERE sent_at > 0 GROUP BY 1 ORDER BY 1`, shift); err != nil {
		return nil, err
	}

	if err := s.eachRow(ctx, func(scan func(...any) error) error {
		var wd, hr, c int
		if err := scan(&wd, &hr, &c); err != nil {
			return err
		}
		if wd >= 0 && wd < 7 && hr >= 0 && hr < 24 {
			st.Heatmap[wd][hr] = c
		}
		return nil
	}, `
		SELECT CAST(strftime('%w', sent_at, 'unixepoch', ?1) AS INTEGER),
			CAST(strftime('%H', sent_at, 'unixepoch', ?1) AS INTEGER), COUNT(*)
		FROM emails WHERE sent_at > 0 GROUP BY 1, 2`, shift); err != nil {
		return nil, err
	}

	var raw []SenderCount
	if err := s.eachRow(ctx, func(scan func(...any) error) error {
		var sc SenderCount
		if err := scan(&sc.Name, &sc.Count); err != nil {
			return err
		}
		raw = append(raw, sc)
		return nil
	}, `SELECT from_addr, COUNT(*) FROM emails WHERE from_addr != '' GROUP BY from_addr`); err != nil {
		return nil, err
	}
	st.TopSenders, st.TopDomains = groupSenders(raw)

	if err := s.eachRow(ctx, func(scan func(...any) error) error {
		var cc CategoryCount
		if err := scan(&cc.CategoryID, &cc.Count); err != nil {
			return err
		}
		st.PerCategory = append(st.PerCategory, cc)
		return nil
	}, `SELECT category_id, COUNT(*) FROM emails WHERE category_id IS NOT NULL GROUP BY category_id`); err != nil {
		return nil, err
	}

	byKind := map[string]KindUsage{}
	if err := s.eachRow(ctx, func(scan func(...any) error) error {
		var k KindUsage
		if err := scan(&k.Kind, &k.Count, &k.Bytes); err != nil {
			return err
		}
		byKind[k.Kind] = k
		st.AttachmentBytes += k.Bytes
		return nil
	}, `SELECT `+kindExpr+`, COUNT(*), IFNULL(SUM(a.size_bytes), 0) FROM attachments a GROUP BY 1`); err != nil {
		return nil, err
	}
	for _, kind := range AttachmentKinds {
		if k, ok := byKind[kind]; ok {
			st.PerKind = append(st.PerKind, k)
		}
	}
	sort.SliceStable(st.PerKind, func(i, j int) bool { return st.PerKind[i].Bytes > st.PerKind[j].Bytes })

	if err := s.eachRow(ctx, func(scan func(...any) error) error {
		var e LargeEmail
		if err := scan(&e.SHA256, &e.Subject, &e.From, &e.SentAt, &e.SizeBytes); err != nil {
			return err
		}
		st.Largest = append(st.Largest, e)
		return nil
	}, fmt.Sprintf(`
		SELECT sha256, IFNULL(subject, ''), IFNULL(from_addr, ''), IFNULL(sent_at, 0), IFNULL(size_bytes, 0)
		FROM emails ORDER BY size_bytes DESC, id LIMIT %d`, largestLimit)); err != nil {
		return nil, err
	}

	return st, nil
}

// eachRow runs query and hands each row's Scan to fn.
func (s *Store) eachRow(ctx context.Context, fn func(scan func(...any) error) error, query string, args ...any) error {
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := fn(rows.Scan); err != nil {
			return err
		}
	}
	return rows.Err()
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
