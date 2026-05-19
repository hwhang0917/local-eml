package store

import (
	"context"
	"time"
)

type Import struct {
	ID         string    `json:"id"`
	SourceKind string    `json:"source_kind"`
	SourceName string    `json:"source_name"`
	Status     string    `json:"status"`
	Total      int       `json:"total"`
	Processed  int       `json:"processed"`
	Duplicates int       `json:"duplicates"`
	Errors     int       `json:"errors"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

func (s *Store) CreateImport(ctx context.Context, imp Import) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO imports (id, source_kind, source_name, status, total, started_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		imp.ID, imp.SourceKind, imp.SourceName, imp.Status, imp.Total, time.Now().Unix())
	return err
}

func (s *Store) SetImportTotal(ctx context.Context, id string, total int) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE imports SET total = ? WHERE id = ?`, total, id)
	return err
}

func (s *Store) UpdateImportStatus(ctx context.Context, id, status string, finished bool) error {
	if finished {
		_, err := s.DB.ExecContext(ctx, `
			UPDATE imports SET status = ?, finished_at = ? WHERE id = ?`,
			status, time.Now().Unix(), id)
		return err
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE imports SET status = ? WHERE id = ?`, status, id)
	return err
}

func (s *Store) IncImportCounters(ctx context.Context, id string, processed, duplicates, errCount int) error {
	_, err := s.DB.ExecContext(ctx, `
		UPDATE imports
		SET processed = processed + ?,
		    duplicates = duplicates + ?,
		    errors = errors + ?
		WHERE id = ?`, processed, duplicates, errCount, id)
	return err
}

func (s *Store) GetImport(ctx context.Context, id string) (*Import, error) {
	var imp Import
	var started, finished int64
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, source_kind, source_name, status, total, processed, duplicates,
			errors, COALESCE(started_at, 0), COALESCE(finished_at, 0)
		FROM imports WHERE id = ?`, id).Scan(
		&imp.ID, &imp.SourceKind, &imp.SourceName, &imp.Status,
		&imp.Total, &imp.Processed, &imp.Duplicates, &imp.Errors,
		&started, &finished)
	if err != nil {
		return nil, err
	}
	if started > 0 {
		imp.StartedAt = time.Unix(started, 0).UTC()
	}
	if finished > 0 {
		imp.FinishedAt = time.Unix(finished, 0).UTC()
	}
	return &imp, nil
}

func (s *Store) RecordImportError(ctx context.Context, id, path, message string) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO import_errors (import_id, path, message)
		VALUES (?, ?, ?)`, id, path, message)
	return err
}
