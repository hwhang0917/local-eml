package server

import (
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// Request limits shared by the upload and restore endpoints.
const (
	maxUploadSize     = 1 << 30
	multipartMemBytes = 32 << 20
	// uploadField is the multipart field name the SPA puts files in.
	uploadField = "file"
	// maxNameLen bounds user-chosen names (categories, saved profiles).
	maxNameLen = 64
	// shaHexLen is the length of a SHA-256 hex digest, which names every blob.
	shaHexLen = 64
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decodeJSON reads the request body into v. On failure it has already
// written the 400, so the caller just returns.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

// writeStoreErr maps a store error to a response: notFound becomes a 404,
// anything else a 500.
func writeStoreErr(w http.ResponseWriter, err, notFound error) {
	if errors.Is(err, notFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// shaParam returns the {sha} route parameter, writing a 400 when it is not a
// SHA-256 hex digest.
func shaParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	sha := chi.URLParam(r, "sha")
	if !validSHA(sha) {
		http.Error(w, "invalid sha", http.StatusBadRequest)
		return "", false
	}
	return sha, true
}

// idParam returns the {id} route parameter as an integer, writing a 400 when
// it is not one.
func idParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

// uploadedFiles parses the multipart body and returns the uploaded files,
// writing the 400 itself when there are none.
func uploadedFiles(w http.ResponseWriter, r *http.Request) ([]*multipart.FileHeader, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(multipartMemBytes); err != nil {
		http.Error(w, "parse multipart: "+err.Error(), http.StatusBadRequest)
		return nil, false
	}
	if r.MultipartForm == nil || len(r.MultipartForm.File[uploadField]) == 0 {
		http.Error(w, "missing '"+uploadField+"' field", http.StatusBadRequest)
		return nil, false
	}
	return r.MultipartForm.File[uploadField], true
}

// copyToTemp spools src into a new temp file named after pattern and returns
// its path. Nothing is left behind on failure.
func copyToTemp(src io.Reader, pattern string) (string, error) {
	tmp, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

func intParam(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func validSHA(s string) bool {
	if len(s) != shaHexLen {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
