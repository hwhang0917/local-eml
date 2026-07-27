package store

import (
	"context"
	"database/sql"
	"errors"
)

// CategoryColors is the fixed palette, in display order. It is the whole set of
// categories: one row per colour, seeded once and thereafter only renamed, the
// way Finder's colour tags work. Mirrored in web/src/lib/categories.ts — this is
// the authority; keep the two in step.
var CategoryColors = []string{"red", "orange", "yellow", "green", "blue", "purple", "grey"}

// Category is a coloured label, at most one of which can be assigned to an
// email. Name is empty until the user picks one, which lets the front end show
// the colour's own name in the current language as the default.
type Category struct {
	ID       int64  `json:"id"`
	Color    string `json:"color"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

var ErrCategoryNotFound = errors.New("category not found")

// seedCategories inserts any missing colour. It runs on every Open and is
// idempotent: existing rows keep their names, and a colour added to the palette
// later simply appears.
func (s *Store) seedCategories(ctx context.Context) error {
	for i, color := range CategoryColors {
		if _, err := s.DB.ExecContext(ctx, `
			INSERT INTO categories (color, name, position) VALUES (?, '', ?)
			ON CONFLICT(color) DO UPDATE SET position = excluded.position`,
			color, i); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, color, name, position FROM categories ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Color, &c.Name, &c.Position); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// RenameCategory is the only write. Colours are the identity and the set is
// fixed, so there is nothing to create, recolour or delete.
func (s *Store) RenameCategory(ctx context.Context, id int64, name string) (Category, error) {
	res, err := s.DB.ExecContext(ctx,
		`UPDATE categories SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return Category{}, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return Category{}, err
	} else if n == 0 {
		return Category{}, ErrCategoryNotFound
	}

	var c Category
	if err := s.DB.QueryRowContext(ctx,
		`SELECT id, color, name, position FROM categories WHERE id = ?`, id).
		Scan(&c.ID, &c.Color, &c.Name, &c.Position); err != nil {
		return Category{}, err
	}
	return c, nil
}

// SetEmailCategory assigns a category, or clears it when categoryID is nil.
//
// The category is checked to exist rather than trusting the foreign key: Open
// runs `PRAGMA foreign_keys = ON` once against the pool, which binds to
// whichever single connection served it, and database/sql opens more on demand
// with the pragma off. Holding eight connections at once reports
// [1 0 0 0 0 0 0 0], so nothing here may depend on the constraint firing.
func (s *Store) SetEmailCategory(ctx context.Context, sha string, categoryID *int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var arg any
	if categoryID != nil {
		var exists int
		err := tx.QueryRowContext(ctx,
			`SELECT 1 FROM categories WHERE id = ?`, *categoryID).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCategoryNotFound
		}
		if err != nil {
			return err
		}
		arg = *categoryID
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE emails SET category_id = ? WHERE sha256 = ?`, arg, sha)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrEmailNotFound
	}
	return tx.Commit()
}
