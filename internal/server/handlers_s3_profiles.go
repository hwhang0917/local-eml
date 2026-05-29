package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/hwhang0917/local-eml/internal/store"
)

type s3ProfileBody struct {
	Name        string  `json:"name"`
	Bucket      string  `json:"bucket"`
	Prefix      *string `json:"prefix,omitempty"`
	Region      *string `json:"region,omitempty"`
	AccessKeyID *string `json:"access_key_id,omitempty"`
}

const maxS3ProfileName = 64

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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	bucket := strings.TrimSpace(body.Bucket)
	if name == "" || len(name) > maxS3ProfileName {
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
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.Store.DeleteS3Profile(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrS3ProfileNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

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
