package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hwhang0917/local-eml/internal/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	dsn := "file:" + filepath.Join(dir, "test.db") + "?_pragma=journal_mode(WAL)"
	s, err := store.Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return &Server{Store: s}
}

func TestIMAPProfilesHandler_SaveListDelete(t *testing.T) {
	s := newTestServer(t)
	r := s.Router()

	body := map[string]any{"name": "Work", "host": "imap.example.com", "port": 993, "username": "u@example.com", "folder": "INBOX"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/imap/profiles", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save: status %d body %s", rec.Code, rec.Body.String())
	}
	var saved store.IMAPProfile
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatalf("decode save body: %v", err)
	}
	if saved.ID == 0 || saved.Name != "Work" {
		t.Fatalf("unexpected saved row: %+v", saved)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/imap/profiles", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: status %d", rec.Code)
	}
	var list []store.IMAPProfile
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 || list[0].ID != saved.ID {
		t.Fatalf("unexpected list: %+v", list)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/imap/profiles/"+strconv.FormatInt(saved.ID, 10), nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestIMAPProfilesHandler_UpdateByName(t *testing.T) {
	s := newTestServer(t)
	r := s.Router()

	b1, _ := json.Marshal(map[string]any{"name": "Work", "host": "old.example.com", "username": "u"})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/imap/profiles", bytes.NewReader(b1)))

	b2, _ := json.Marshal(map[string]any{"name": "Work", "host": "new.example.com", "username": "u2"})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/imap/profiles", bytes.NewReader(b2)))
	if rec.Code != http.StatusOK {
		t.Fatalf("second save: status %d body %s", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/imap/profiles", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var list []store.IMAPProfile
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("expected 1 row, got %d", len(list))
	}
	if list[0].Host != "new.example.com" || list[0].Username != "u2" {
		t.Fatalf("update not applied: %+v", list[0])
	}
}

func TestIMAPProfilesHandler_Validation(t *testing.T) {
	s := newTestServer(t)
	r := s.Router()

	cases := []struct {
		name string
		body map[string]any
		want int
	}{
		{"blank name", map[string]any{"name": "", "host": "h", "username": "u"}, http.StatusBadRequest},
		{"blank host", map[string]any{"name": "N", "host": "", "username": "u"}, http.StatusBadRequest},
		{"blank username", map[string]any{"name": "N", "host": "h", "username": ""}, http.StatusBadRequest},
		{"port too low", map[string]any{"name": "N", "host": "h", "username": "u", "port": 0}, http.StatusBadRequest},
		{"port too high", map[string]any{"name": "N", "host": "h", "username": "u", "port": 70000}, http.StatusBadRequest},
		{"name too long", map[string]any{"name": strings.Repeat("a", 65), "host": "h", "username": "u"}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.body)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/imap/profiles", bytes.NewReader(b)))
			if rec.Code != tc.want {
				t.Fatalf("status: want %d got %d body=%s", tc.want, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestIMAPProfilesHandler_DeleteNotFound(t *testing.T) {
	s := newTestServer(t)
	r := s.Router()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/imap/profiles/999", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/imap/profiles/notanumber", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on non-numeric id, got %d", rec.Code)
	}
}
