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
	// Endpoint is set for S3-compatible services; nil means AWS.
	Endpoint  *string `json:"endpoint,omitempty"`
	PathStyle bool    `json:"path_style"`
	CreatedAt int64   `json:"created_at"`
	UpdatedAt int64   `json:"updated_at"`
}

var ErrS3ProfileNotFound = errors.New("s3 profile not found")

func (s *Store) ListS3Profiles(ctx context.Context) ([]S3Profile, error) {
	return queryAll(ctx, s.DB, scanS3Profile, `
		SELECT id, name, bucket, prefix, region, access_key_id, endpoint, path_style,
		       created_at, updated_at
		FROM s3_profiles
		ORDER BY name COLLATE NOCASE`)
}

func scanS3Profile(rs rowScanner) (S3Profile, error) {
	var p S3Profile
	var prefix, region, accessKey, endpoint sql.Null[string]
	if err := rs.Scan(&p.ID, &p.Name, &p.Bucket, &prefix, &region, &accessKey,
		&endpoint, &p.PathStyle, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return S3Profile{}, err
	}
	p.Prefix = nullPtr(prefix)
	p.Region = nullPtr(region)
	p.AccessKeyID = nullPtr(accessKey)
	p.Endpoint = nullPtr(endpoint)
	return p, nil
}

func (s *Store) UpsertS3Profile(ctx context.Context, p S3Profile) (S3Profile, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return S3Profile{}, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()
	prefixVal := nullable(p.Prefix)
	regionVal := nullable(p.Region)
	accessKeyVal := nullable(p.AccessKeyID)
	endpointVal := nullable(p.Endpoint)

	var existingID int64
	var existingCreated int64
	err = tx.QueryRowContext(ctx,
		`SELECT id, created_at FROM s3_profiles WHERE name = ?`, p.Name,
	).Scan(&existingID, &existingCreated)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := tx.ExecContext(ctx, `
			INSERT INTO s3_profiles
			  (name, bucket, prefix, region, access_key_id, endpoint, path_style, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.Name, p.Bucket, prefixVal, regionVal, accessKeyVal, endpointVal, p.PathStyle, now, now)
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
			UPDATE s3_profiles SET bucket = ?, prefix = ?, region = ?, access_key_id = ?,
			  endpoint = ?, path_style = ?, updated_at = ?
			WHERE id = ?`,
			p.Bucket, prefixVal, regionVal, accessKeyVal, endpointVal, p.PathStyle, now, existingID); err != nil {
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
	return requireRow(res, ErrS3ProfileNotFound)
}
