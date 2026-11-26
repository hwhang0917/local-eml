package store

import (
	"context"
	"time"
)

// Import job statuses. Everything but ImportRunning is terminal.
const (
	ImportQueued  = "queued"
	ImportRunning = "running"
	ImportDone    = "done"
	ImportError   = "error"
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

// UpdateImportStatus records status; a terminal status also stamps finished_at.
func (s *Store) UpdateImportStatus(ctx context.Context, id, status string) error {
	if status == ImportRunning {
		_, err := s.DB.ExecContext(ctx, `UPDATE imports SET status = ? WHERE id = ?`, status, id)
		return err
	}
	_, err := s.DB.ExecContext(ctx, `
		UPDATE imports SET status = ?, finished_at = ? WHERE id = ?`,
		status, time.Now().Unix(), id)
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

// importCols is the projection scanImport expects, in order.
const importCols = `id, source_kind, source_name, status, total, processed, duplicates,
	errors, COALESCE(started_at, 0), COALESCE(finished_at, 0)`

func scanImport(rs rowScanner) (Import, error) {
	var imp Import
	var started, finished int64
	if err := rs.Scan(
		&imp.ID, &imp.SourceKind, &imp.SourceName, &imp.Status,
		&imp.Total, &imp.Processed, &imp.Duplicates, &imp.Errors,
		&started, &finished); err != nil {
		return Import{}, err
	}
	imp.StartedAt = timeFromUnix(started)
	imp.FinishedAt = timeFromUnix(finished)
	return imp, nil
}

func (s *Store) GetImport(ctx context.Context, id string) (*Import, error) {
	imp, err := scanImport(s.DB.QueryRowContext(ctx,
		`SELECT `+importCols+` FROM imports WHERE id = ?`, id))
	if err != nil {
		return nil, err
	}
	return &imp, nil
}

// ListFinishedImports returns the jobs of one kind that reached a terminal
// status at or after since (Unix seconds), oldest first, at most limit of
// them. The SPA polls it for background syncs that brought in new mail.
func (s *Store) ListFinishedImports(ctx context.Context, kind string, since int64, limit int) ([]Import, error) {
	return queryAll(ctx, s.DB, scanImport, `
		SELECT `+importCols+` FROM imports
		WHERE source_kind = ? AND finished_at IS NOT NULL AND finished_at >= ?
		ORDER BY finished_at, id
		LIMIT ?`, kind, since, limit)
}

func (s *Store) RecordImportError(ctx context.Context, id, path, message string) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO import_errors (import_id, path, message)
		VALUES (?, ?, ?)`, id, path, message)
	return err
}
