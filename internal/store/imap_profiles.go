package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type IMAPProfile struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Host         string  `json:"host"`
	Port         *int    `json:"port,omitempty"`
	Username     string  `json:"username"`
	Folder       *string `json:"folder,omitempty"`
	SyncEnabled  bool    `json:"sync_enabled"`
	UIDValidity  *uint32 `json:"uid_validity,omitempty"`
	LastUID      *uint32 `json:"last_uid,omitempty"`
	LastSyncedAt *int64  `json:"last_synced_at,omitempty"`
	HasPassword  bool    `json:"has_password"`
	CreatedAt    int64   `json:"created_at"`
	UpdatedAt    int64   `json:"updated_at"`
}

var ErrIMAPProfileNotFound = errors.New("imap profile not found")

func (s *Store) ListIMAPProfiles(ctx context.Context) ([]IMAPProfile, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, name, host, port, username, folder,
			sync_enabled, uid_validity, last_uid, last_synced_at,
			encrypted_password IS NOT NULL AND length(encrypted_password) > 0,
			created_at, updated_at
		FROM imap_profiles
		ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IMAPProfile{}
	for rows.Next() {
		p, err := scanIMAPProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetIMAPProfile(ctx context.Context, id int64) (*IMAPProfile, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, name, host, port, username, folder,
			sync_enabled, uid_validity, last_uid, last_synced_at,
			encrypted_password IS NOT NULL AND length(encrypted_password) > 0,
			created_at, updated_at
		FROM imap_profiles WHERE id = ?`, id)
	p, err := scanIMAPProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrIMAPProfileNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListIMAPProfilesForSync returns every profile flagged sync_enabled=1 that
// also has a stored encrypted password — the precondition for unattended sync.
func (s *Store) ListIMAPProfilesForSync(ctx context.Context) ([]IMAPProfile, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, name, host, port, username, folder,
			sync_enabled, uid_validity, last_uid, last_synced_at,
			1, created_at, updated_at
		FROM imap_profiles
		WHERE sync_enabled = 1
		  AND encrypted_password IS NOT NULL
		  AND length(encrypted_password) > 0
		ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IMAPProfile{}
	for rows.Next() {
		p, err := scanIMAPProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetIMAPProfilePassword(ctx context.Context, id int64) ([]byte, error) {
	var blob []byte
	err := s.DB.QueryRowContext(ctx,
		`SELECT encrypted_password FROM imap_profiles WHERE id = ?`, id).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrIMAPProfileNotFound
	}
	return blob, err
}

func (s *Store) SetIMAPProfilePassword(ctx context.Context, id int64, blob []byte) error {
	res, err := s.DB.ExecContext(ctx,
		`UPDATE imap_profiles SET encrypted_password = ?, updated_at = ? WHERE id = ?`,
		blob, time.Now().Unix(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrIMAPProfileNotFound
	}
	return nil
}

func (s *Store) UpdateIMAPProfileSyncState(ctx context.Context, id int64,
	uidValidity, lastUID uint32, syncedAt int64) error {
	res, err := s.DB.ExecContext(ctx, `
		UPDATE imap_profiles
		SET uid_validity = ?, last_uid = ?, last_synced_at = ?, updated_at = ?
		WHERE id = ?`,
		uidValidity, lastUID, syncedAt, time.Now().Unix(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrIMAPProfileNotFound
	}
	return nil
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
	syncFlag := 0
	if p.SyncEnabled {
		syncFlag = 1
	}

	var existingID, existingCreated int64
	err = tx.QueryRowContext(ctx,
		`SELECT id, created_at FROM imap_profiles WHERE name = ?`, p.Name,
	).Scan(&existingID, &existingCreated)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := tx.ExecContext(ctx, `
			INSERT INTO imap_profiles
			  (name, host, port, username, folder, sync_enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			p.Name, p.Host, portVal, p.Username, folderVal, syncFlag, now, now)
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
			UPDATE imap_profiles
			SET host = ?, port = ?, username = ?, folder = ?,
			    sync_enabled = ?, updated_at = ?
			WHERE id = ?`,
			p.Host, portVal, p.Username, folderVal, syncFlag, now, existingID); err != nil {
			return IMAPProfile{}, err
		}
		// Disabling sync revokes the stored password + sync cursor. The README
		// promises this behavior; failing to drop the secret here would leave
		// an unused but still-decryptable blob in the database.
		if !p.SyncEnabled {
			if _, err := tx.ExecContext(ctx, `
				UPDATE imap_profiles
				SET encrypted_password = NULL,
				    uid_validity = NULL,
				    last_uid = NULL,
				    last_synced_at = NULL
				WHERE id = ?`, existingID); err != nil {
				return IMAPProfile{}, err
			}
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

type imapProfileScanner interface {
	Scan(dest ...any) error
}

func scanIMAPProfile(rs imapProfileScanner) (IMAPProfile, error) {
	var p IMAPProfile
	var port sql.NullInt64
	var folder sql.NullString
	var syncFlag int
	var uidValidity, lastUID sql.NullInt64
	var lastSyncedAt sql.NullInt64
	var hasPassword bool
	if err := rs.Scan(&p.ID, &p.Name, &p.Host, &port, &p.Username, &folder,
		&syncFlag, &uidValidity, &lastUID, &lastSyncedAt, &hasPassword,
		&p.CreatedAt, &p.UpdatedAt); err != nil {
		return IMAPProfile{}, err
	}
	if port.Valid {
		v := int(port.Int64)
		p.Port = &v
	}
	if folder.Valid {
		v := folder.String
		p.Folder = &v
	}
	p.SyncEnabled = syncFlag != 0
	if uidValidity.Valid {
		v := uint32(uidValidity.Int64)
		p.UIDValidity = &v
	}
	if lastUID.Valid {
		v := uint32(lastUID.Int64)
		p.LastUID = &v
	}
	if lastSyncedAt.Valid {
		v := lastSyncedAt.Int64
		p.LastSyncedAt = &v
	}
	p.HasPassword = hasPassword
	return p, nil
}
