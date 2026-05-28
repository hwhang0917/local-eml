package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type IMAPProfile struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Host      string  `json:"host"`
	Port      *int    `json:"port,omitempty"`
	Username  string  `json:"username"`
	Folder    *string `json:"folder,omitempty"`
	CreatedAt int64   `json:"created_at"`
	UpdatedAt int64   `json:"updated_at"`
}

var ErrIMAPProfileNotFound = errors.New("imap profile not found")

func (s *Store) ListIMAPProfiles(ctx context.Context) ([]IMAPProfile, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, name, host, port, username, folder, created_at, updated_at
		FROM imap_profiles
		ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IMAPProfile{}
	for rows.Next() {
		var p IMAPProfile
		var port sql.NullInt64
		var folder sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.Host, &port, &p.Username, &folder, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if port.Valid {
			v := int(port.Int64)
			p.Port = &v
		}
		if folder.Valid {
			v := folder.String
			p.Folder = &v
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) UpsertIMAPProfile(ctx context.Context, p IMAPProfile) (IMAPProfile, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return IMAPProfile{}, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()
	var portVal any
	if p.Port != nil {
		portVal = *p.Port
	}
	var folderVal any
	if p.Folder != nil {
		folderVal = *p.Folder
	}

	var existingID int64
	var existingCreated int64
	err = tx.QueryRowContext(ctx,
		`SELECT id, created_at FROM imap_profiles WHERE name = ?`, p.Name,
	).Scan(&existingID, &existingCreated)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := tx.ExecContext(ctx, `
			INSERT INTO imap_profiles (name, host, port, username, folder, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			p.Name, p.Host, portVal, p.Username, folderVal, now, now)
		if err != nil {
			return IMAPProfile{}, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return IMAPProfile{}, err
		}
		p.ID = id
		p.CreatedAt = now
		p.UpdatedAt = now
	case err == nil:
		if _, err := tx.ExecContext(ctx, `
			UPDATE imap_profiles SET host = ?, port = ?, username = ?, folder = ?, updated_at = ?
			WHERE id = ?`,
			p.Host, portVal, p.Username, folderVal, now, existingID); err != nil {
			return IMAPProfile{}, err
		}
		p.ID = existingID
		p.CreatedAt = existingCreated
		p.UpdatedAt = now
	default:
		return IMAPProfile{}, err
	}

	if err := tx.Commit(); err != nil {
		return IMAPProfile{}, err
	}
	return p, nil
}

func (s *Store) DeleteIMAPProfile(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM imap_profiles WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrIMAPProfileNotFound
	}
	return nil
}
