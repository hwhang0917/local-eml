package server

import (
	"net/http"
	"strings"

	"github.com/hwhang0917/local-eml/internal/store"
)

type s3ProfileBody struct {
	Name        string  `json:"name"`
	Bucket      string  `json:"bucket"`
	Prefix      *string `json:"prefix,omitempty"`
	Region      *string `json:"region,omitempty"`
	AccessKeyID *string `json:"access_key_id,omitempty"`
}

func (s *Server) handleListS3Profiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := s.Store.ListS3Profiles(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, profiles)
}

func (s *Server) handleSaveS3Profile(w http.ResponseWriter, r *http.Request) {
	var body s3ProfileBody
	if !decodeJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	bucket := strings.TrimSpace(body.Bucket)
	if !validName(name) {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}
	if bucket == "" {
		http.Error(w, "invalid bucket", http.StatusBadRequest)
		return
	}
	saved, err := s.Store.UpsertS3Profile(r.Context(), store.S3Profile{
		Name:        name,
		Bucket:      bucket,
		Prefix:      trimmedOrNil(body.Prefix),
		Region:      trimmedOrNil(body.Region),
		AccessKeyID: trimmedOrNil(body.AccessKeyID),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleDeleteS3Profile(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	if err := s.Store.DeleteS3Profile(r.Context(), id); err != nil {
		writeStoreErr(w, err, store.ErrS3ProfileNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// validName accepts a user-chosen profile or category name.
func validName(name string) bool {
	return name != "" && len(name) <= maxNameLen
}

// trimmedOrNil treats an absent or blank optional field as unset.
func trimmedOrNil(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}
