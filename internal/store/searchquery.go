package store

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Search syntax. The search box takes plain words plus a few Gmail-style
// operators, so one line can say "the receipt from Amazon in 2019 with a PDF":
//
//	amazon receipt filename:pdf after:2019 before:2020
//
// Plain words match the subject, sender, recipients, body and attachment file
// names; every word must match somewhere (AND). "Quoted words" match as an
// exact phrase. A leading - excludes. Operators:
//
//	from:  to:  cc:  subject:  filename:   text in that field only
//	has:attachment                          messages with attachments
//	is:starred  is:unread  is:read          message state
//	after:  before:  on:                    YYYY, YYYY-MM or YYYY-MM-DD
//	larger:  smaller:                       size, e.g. 500K, 2M, 1G
//
// Anything that doesn't parse as an operator (an unknown key, a bad date, an
// empty value while the person is still typing) is searched as plain text or
// dropped, never an error: a half-typed query should not blank the library.

type searchClause struct {
	// field is "" for a plain term, else the operator key.
	field  string
	value  string
	phrase bool
	negate bool
}

// searchOperators are the keys tokenize recognises. Anything else with a colon
// (a URL, a time like 10:30) stays a plain term.
var searchOperators = map[string]bool{
	"from": true, "to": true, "cc": true, "subject": true, "filename": true,
	"has": true, "is": true, "after": true, "before": true, "on": true,
	"larger": true, "smaller": true,
}

// parseSearch splits a raw query into clauses. Keys are case-insensitive.
func parseSearch(raw string) []searchClause {
	var out []searchClause
	rs := []rune(raw)
	i := 0
	for i < len(rs) {
		for i < len(rs) && unicode.IsSpace(rs[i]) {
			i++
		}
		if i >= len(rs) {
			break
		}
		c := searchClause{}
		// A lone "-" or one before a space is a literal, not a negation.
		if rs[i] == '-' && i+1 < len(rs) && !unicode.IsSpace(rs[i+1]) {
			c.negate = true
			i++
		}
		// Operator key: letters up to a colon, if the key is one we know.
		j := i
		for j < len(rs) && unicode.IsLetter(rs[j]) {
			j++
		}
		if j < len(rs) && rs[j] == ':' && j > i && searchOperators[strings.ToLower(string(rs[i:j]))] {
			c.field = strings.ToLower(string(rs[i:j]))
			i = j + 1
		}
		if i < len(rs) && rs[i] == '"' {
			end := i + 1
			for end < len(rs) && rs[end] != '"' {
				end++
			}
			c.value = string(rs[i+1 : end])
			c.phrase = true
			i = end + 1
		} else {
			end := i
			for end < len(rs) && !unicode.IsSpace(rs[end]) {
				end++
			}
			c.value = string(rs[i:end])
			i = end
		}
		c.value = strings.TrimSpace(c.value)
		if c.value == "" {
			continue
		}
		out = append(out, c)
	}
	return out
}

// searchConds turns a raw query into WHERE predicates on emails (never a
// JOIN, see listWhere). Dates are read in loc, as the date picker's are.
func searchConds(raw string, loc *time.Location) ([]string, []any) {
	var conds []string
	var args []any
	add := func(negate bool, cond string, a ...any) {
		if negate {
			cond = "NOT " + cond
		}
		conds = append(conds, cond)
		args = append(args, a...)
	}
	for _, c := range parseSearch(raw) {
		switch c.field {
		case "":
			if hasJamo(c.value) {
				add(c.negate, `(chosung_text LIKE ?)`, "%"+toChosung(c.value)+"%")
				continue
			}
			add(c.negate, `(id IN (SELECT rowid FROM emails_fts WHERE emails_fts MATCH ?)
				OR id IN (SELECT email_id FROM attachments WHERE filename LIKE ? ESCAPE '\'))`,
				ftsPhrase(c.value, c.phrase), likeContains(c.value))
		case "from", "subject":
			if hasJamo(c.value) {
				add(c.negate, `(chosung_text LIKE ?)`, "%"+toChosung(c.value)+"%")
				continue
			}
			col := "from_addr"
			if c.field == "subject" {
				col = "subject"
			}
			add(c.negate, `(id IN (SELECT rowid FROM emails_fts WHERE emails_fts MATCH ?))`,
				col+" : "+ftsPhrase(c.value, c.phrase))
		case "to":
			// Like Gmail, to: also covers Cc. Cc isn't in the FTS index, so it's
			// a substring match on the stored JSON list.
			add(c.negate, `(id IN (SELECT rowid FROM emails_fts WHERE emails_fts MATCH ?)
				OR cc_addrs LIKE ? ESCAPE '\')`,
				"to_addrs : "+ftsPhrase(c.value, c.phrase), likeContains(c.value))
		case "cc":
			add(c.negate, `(cc_addrs LIKE ? ESCAPE '\')`, likeContains(c.value))
		case "filename":
			add(c.negate, `(id IN (SELECT email_id FROM attachments WHERE filename LIKE ? ESCAPE '\'))`,
				likeContains(c.value))
		case "has":
			// has:attach, has:attachment and has:attachments all work.
			if v := strings.ToLower(c.value); len(v) >= 6 && strings.HasPrefix("attachments", v) {
				add(c.negate, `(has_attachments = 1)`)
			}
		case "is":
			switch strings.ToLower(c.value) {
			case "starred":
				add(c.negate, `(starred = 1)`)
			case "unread":
				add(c.negate, `(read = 0)`)
			case "read":
				add(c.negate, `(read = 1)`)
			}
		case "after", "before", "on":
			start, end, ok := parsePeriod(c.value, loc)
			if !ok {
				continue
			}
			// after: and on: start at the period's first second; before: ends
			// just before it, so after:2019 before:2020 is exactly 2019.
			var cond string
			var a []any
			switch c.field {
			case "after":
				cond, a = `sent_at >= ?`, []any{start}
			case "before":
				cond, a = `(sent_at > 0 AND sent_at < ?)`, []any{start}
			case "on":
				cond, a = `(sent_at >= ? AND sent_at < ?)`, []any{start, end}
			}
			add(c.negate, cond, a...)
		case "larger", "smaller":
			n, ok := parseSize(c.value)
			if !ok {
				continue
			}
			op := ">="
			if c.field == "smaller" {
				op = "<"
			}
			add(c.negate, `(size_bytes `+op+` ?)`, n)
		}
	}
	return conds, args
}

// ftsPhrase quotes a term for FTS5. A bare word is a prefix match, as the
// search box always did; a quoted one must match exactly.
func ftsPhrase(v string, exact bool) string {
	q := `"` + strings.ReplaceAll(v, `"`, `""`) + `"`
	if !exact {
		q += "*"
	}
	return q
}

// likeContains builds a LIKE pattern matching v anywhere, with the wildcard
// characters in v taken literally (paired with ESCAPE '\').
func likeContains(v string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(v) + "%"
}

// parsePeriod reads YYYY, YYYY-MM or YYYY-MM-DD (slashes or dots also work)
// as a span of local time, returned as [start, end) unix seconds.
func parsePeriod(v string, loc *time.Location) (int64, int64, bool) {
	v = strings.NewReplacer("/", "-", ".", "-").Replace(v)
	parts := strings.Split(v, "-")
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return 0, 0, false
		}
		nums[i] = n
	}
	switch len(nums) {
	case 1:
		if nums[0] < 1970 || nums[0] > 9999 {
			return 0, 0, false
		}
		s := time.Date(nums[0], 1, 1, 0, 0, 0, 0, loc)
		return s.Unix(), s.AddDate(1, 0, 0).Unix(), true
	case 2:
		if nums[0] < 1970 || nums[1] < 1 || nums[1] > 12 {
			return 0, 0, false
		}
		s := time.Date(nums[0], time.Month(nums[1]), 1, 0, 0, 0, 0, loc)
		return s.Unix(), s.AddDate(0, 1, 0).Unix(), true
	case 3:
		if nums[0] < 1970 || nums[1] < 1 || nums[1] > 12 || nums[2] < 1 || nums[2] > 31 {
			return 0, 0, false
		}
		s := time.Date(nums[0], time.Month(nums[1]), nums[2], 0, 0, 0, 0, loc)
		// time.Date normalises Feb 31 to Mar 3; reject instead of surprising.
		if s.Day() != nums[2] {
			return 0, 0, false
		}
		return s.Unix(), s.AddDate(0, 0, 1).Unix(), true
	}
	return 0, 0, false
}

// parseSize reads a byte count with an optional K, M or G suffix (KB, MB, GB
// too), binary multiples as file managers show them.
func parseSize(v string) (int64, bool) {
	v = strings.ToUpper(strings.TrimSpace(v))
	v = strings.TrimSuffix(v, "B")
	mult := int64(1)
	switch {
	case strings.HasSuffix(v, "K"):
		mult, v = 1<<10, strings.TrimSuffix(v, "K")
	case strings.HasSuffix(v, "M"):
		mult, v = 1<<20, strings.TrimSuffix(v, "M")
	case strings.HasSuffix(v, "G"):
		mult, v = 1<<30, strings.TrimSuffix(v, "G")
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 {
		return 0, false
	}
	return int64(f * float64(mult)), true
}
