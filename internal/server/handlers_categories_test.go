package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/hwhang0917/local-eml/internal/store"
)

func listCategories(t *testing.T, s *Server) []store.Category {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodGet, "/api/categories", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d, want 200", rec.Code)
	}
	var list []store.Category
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return list
}

func renameCategory(t *testing.T, s *Server, id int64, name string) int {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"name": name})
	rec := httptest.NewRecorder()
	req := newLoopbackRequest(http.MethodPut,
		"/api/categories/"+strconv.FormatInt(id, 10), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rec, req)
	return rec.Code
}

func TestCategoriesHandlerListsSeededPalette(t *testing.T) {
	s := newTestServer(t)
	list := listCategories(t, s)
	if len(list) != len(store.CategoryColors) {
		t.Fatalf("got %d categories, want %d", len(list), len(store.CategoryColors))
	}
	if list[0].Color != store.CategoryColors[0] {
		t.Errorf("first colour = %q, want %q", list[0].Color, store.CategoryColors[0])
	}
}

func TestCategoriesHandlerRename(t *testing.T) {
	s := newTestServer(t)
	list := listCategories(t, s)

	if code := renameCategory(t, s, list[0].ID, "  Work  "); code != http.StatusOK {
		t.Fatalf("rename = %d, want 200", code)
	}
	if got := listCategories(t, s)[0].Name; got != "Work" {
		t.Errorf("name = %q, want trimmed %q", got, "Work")
	}

	// Empty is valid: it restores the colour's own name.
	if code := renameCategory(t, s, list[0].ID, ""); code != http.StatusOK {
		t.Errorf("clear = %d, want 200", code)
	}
	if got := listCategories(t, s)[0].Name; got != "" {
		t.Errorf("name = %q, want empty", got)
	}

	if code := renameCategory(t, s, list[0].ID, strings.Repeat("a", 65)); code != http.StatusBadRequest {
		t.Errorf("overlong name = %d, want 400", code)
	}
	if code := renameCategory(t, s, list[0].ID+9999, "X"); code != http.StatusNotFound {
		t.Errorf("unknown id = %d, want 404", code)
	}
}

// The set is fixed, so there must be no way to add an eighth colour or remove
// one of the seven.
func TestCategoriesCannotBeCreatedOrDeleted(t *testing.T) {
	s := newTestServer(t)
	id := listCategories(t, s)[0].ID

	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/categories"},
		{http.MethodDelete, "/api/categories/" + strconv.FormatInt(id, 10)},
	} {
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, newLoopbackRequest(tc.method, tc.path, bytes.NewReader([]byte(`{}`))))
		if rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusNotFound {
			t.Errorf("%s %s = %d, want the route not to exist", tc.method, tc.path, rec.Code)
		}
	}
	if n := len(listCategories(t, s)); n != len(store.CategoryColors) {
		t.Errorf("palette changed: %d rows", n)
	}
}

func TestEmailCategoryAssignAndClear(t *testing.T) {
	s, p := newDriftServer(t)
	sha := writeBlob(t, p)
	if _, err := s.Store.InsertEmail(context.Background(), store.EmailRow{
		Email: store.Email{SHA256: sha, Subject: "x"},
	}); err != nil {
		t.Fatal(err)
	}
	cat := listCategories(t, s)[0]
	idPath := "/api/emails/" + sha + "/category/"

	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(
		http.MethodPut, idPath+strconv.FormatInt(cat.ID+9999, 10), nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("assign unknown category = %d, want 400", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(
		http.MethodPut, idPath+strconv.FormatInt(cat.ID, 10), nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("assign = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}

	_, body := getEmail(t, s, sha)
	if got := body["category_id"]; got != float64(cat.ID) {
		t.Errorf("category_id = %v, want %d", got, cat.ID)
	}

	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(
		http.MethodDelete, "/api/emails/"+sha+"/category", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("clear = %d, want 204", rec.Code)
	}
	_, body = getEmail(t, s, sha)
	if _, present := body["category_id"]; present {
		t.Errorf("category_id still serialized after clear: %v", body["category_id"])
	}
}

func TestListEmailsCategoryQueryParam(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()
	cat := listCategories(t, s)[0]
	for _, sha := range []string{strings.Repeat("a", 64), strings.Repeat("b", 64)} {
		if _, err := s.Store.InsertEmail(ctx, store.EmailRow{
			Email: store.Email{SHA256: sha},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Store.SetEmailCategory(ctx, strings.Repeat("a", 64), &cat.ID); err != nil {
		t.Fatal(err)
	}

	totalFor := func(query string) int {
		t.Helper()
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodGet, "/api/emails"+query, nil))
		var body struct {
			Total int `json:"total"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return body.Total
	}

	if n := totalFor("?category=" + strconv.FormatInt(cat.ID, 10)); n != 1 {
		t.Errorf("by category total = %d, want 1", n)
	}
	if n := totalFor("?category=none"); n != 1 {
		t.Errorf("uncategorized total = %d, want 1", n)
	}
	// Junk must not blank the library.
	if n := totalFor("?category=notanumber"); n != 2 {
		t.Errorf("junk filter total = %d, want 2 (treated as any)", n)
	}
}
