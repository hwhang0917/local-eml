package store

import (
	"context"
	"database/sql"
	"strings"
)

type rowScanner interface {
	Scan(dest ...any) error
}

// queryAll runs query and scans every row with scan. The result is never nil,
// so an empty listing encodes as [] rather than null.
func queryAll[T any](ctx context.Context, db *sql.DB, scan func(rowScanner) (T, error), query string, args ...any) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// nullPtr is the inverse of nullable: SQL NULL becomes a nil pointer.
func nullPtr[T any](n sql.Null[T]) *T {
	if !n.Valid {
		return nil
	}
	return &n.V
}

// nullable binds an optional field as SQL NULL when it is unset.
func nullable[T any](v *T) any {
	if v == nil {
		return nil
	}
	return *v
}

// sqlOrder maps a requested sort direction to SQL, newest/largest first
// unless "asc" is asked for.
func sqlOrder(order string) string {
	if strings.EqualFold(order, "asc") {
		return "ASC"
	}
	return "DESC"
}
