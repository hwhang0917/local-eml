package server

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hwhang0917/local-eml/internal/exporter"
)

const sqliteMagic = "SQLite format 3\x00"

// handleRestore merges metadata (stars, categories, settings, profiles) from
// an uploaded backup — either a bare local-eml.db snapshot or a full export
// zip containing one — into the live database. Synchronous: a metadata merge
// is small even for large libraries, so no job/SSE machinery.
func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(multipartMemBytes); err != nil {
		http.Error(w, "parse multipart: "+err.Error(), http.StatusBadRequest)
		return
	}
	if r.MultipartForm == nil || len(r.MultipartForm.File["file"]) != 1 {
		http.Error(w, "exactly one 'file' field required", http.StatusBadRequest)
		return
	}
	hdr := r.MultipartForm.File["file"][0]

	upload, err := saveOne(hdr)
	if err != nil {
		http.Error(w, "save upload: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(upload)

	dbPath, err := extractSnapshot(upload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if dbPath != upload {
		defer os.Remove(dbPath)
	}

	sum, err := s.Store.RestoreMetadata(r.Context(), dbPath)
	if err != nil {
		http.Error(w, "restore: "+err.Error(), http.StatusBadRequest)
		return
	}
	slog.Info("metadata restore complete",
		slog.String("source", hdr.Filename),
		slog.Int64("emails", sum.Emails),
		slog.Int64("categories", sum.Categories),
		slog.Int64("settings", sum.Settings),
		slog.Int64("imap_profiles", sum.ImapProfiles),
		slog.Int64("s3_profiles", sum.S3Profiles),
	)
	writeJSON(w, http.StatusOK, sum)
}

// extractSnapshot returns a path to the SQLite snapshot inside the upload:
// the upload itself for a bare .db file, or the extracted local-eml.db entry
// for an export zip.
func extractSnapshot(upload string) (string, error) {
	head := make([]byte, len(sqliteMagic))
	f, err := os.Open(upload)
	if err != nil {
		return "", err
	}
	n, _ := io.ReadFull(f, head)
	f.Close()
	head = head[:n]

	if bytes.Equal(head, []byte(sqliteMagic)) {
		return upload, nil
	}
	if !bytes.HasPrefix(head, []byte("PK")) {
		return "", fmt.Errorf("not a SQLite database or export zip")
	}

	zr, err := zip.OpenReader(upload)
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()
	for _, ze := range zr.File {
		if ze.Name != exporter.DBObjectName {
			continue
		}
		src, err := ze.Open()
		if err != nil {
			return "", err
		}
		defer src.Close()
		dst, err := os.CreateTemp("", "local-eml-restore-*.db")
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			os.Remove(dst.Name())
			return "", err
		}
		if err := dst.Close(); err != nil {
			os.Remove(dst.Name())
			return "", err
		}
		return dst.Name(), nil
	}
	return "", fmt.Errorf("zip has no %s entry — was it exported by an older version?", filepath.Base(exporter.DBObjectName))
}
