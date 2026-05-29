package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type S3Profile struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Bucket      string  `json:"bucket"`
	Prefix      *string `json:"prefix,omitempty"`
	Region      *string `json:"region,omitempty"`
	AccessKeyID *string `json:"access_key_id,omitempty"`
	CreatedAt   int64   `json:"created_at"`
	UpdatedAt   int64   `json:"updated_at"`
}

var ErrS3ProfileNotFound = errors.New("s3 profile not found")

func (s *Store) ListS3Profiles(ctx context.Context) ([]S3Profile, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, name, bucket, prefix, region, access_key_id, created_at, updated_at
		FROM s3_profiles
		ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []S3Profile{}
	for rows.Next() {
		var p S3Profile
		var prefix, region, accessKey sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.Bucket, &prefix, &region, &accessKey,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if prefix.Valid {
			v := prefix.String
			p.Prefix = &v
		}
		if region.Valid {
			v := region.String
			p.Region = &v
		}
		if accessKey.Valid {
			v := accessKey.String
			p.AccessKeyID = &v
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) UpsertS3Profile(ctx context.Context, p S3Profile) (S3Profile, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return S3Profile{}, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()
	prefixVal := nullableString(p.Prefix)
	regionVal := nullableString(p.Region)
	accessKeyVal := nullableString(p.AccessKeyID)

	var existingID int64
	var existingCreated int64
	err = tx.QueryRowContext(ctx,
		`SELECT id, created_at FROM s3_profiles WHERE name = ?`, p.Name,
	).Scan(&existingID, &existingCreated)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := tx.ExecContext(ctx, `
			INSERT INTO s3_profiles (name, bucket, prefix, region, access_key_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			p.Name, p.Bucket, prefixVal, regionVal, accessKeyVal, now, now)
		if err != nil {
			return S3Profile{}, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return S3Profile{}, err
		}
		p.ID = id
		p.CreatedAt = now
		p.UpdatedAt = now
	case err == nil:
		if _, err := tx.ExecContext(ctx, `
			UPDATE s3_profiles SET bucket = ?, prefix = ?, region = ?, access_key_id = ?, updated_at = ?
			WHERE id = ?`,
			p.Bucket, prefixVal, regionVal, accessKeyVal, now, existingID); err != nil {
			return S3Profile{}, err
		}
		p.ID = existingID
		p.CreatedAt = existingCreated
		p.UpdatedAt = now
	default:
		return S3Profile{}, err
	}

	if err := tx.Commit(); err != nil {
		return S3Profile{}, err
	}
	return p, nil
}

func (s *Store) DeleteS3Profile(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM s3_profiles WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrS3ProfileNotFound
	}
	return nil
}

func nullableString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
