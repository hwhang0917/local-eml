# Separate Detail Page + Named IMAP Profiles Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Revert the in-pane master-detail layout so `/email/:sha` is a normal navigated page again, and add a named-IMAP-profile feature (host/port/username/folder persisted; password never persisted) to the Import page.

**Architecture:** Frontend-only for the layout revert: drop the reka-ui Splitter wrapper, the `?sel` query param, and the chrome-less viewer mode; row click becomes `router.push({ name: 'viewer' })`. Library filter/search/sort/page already round-trip through `route.query`, so browser back restores the prior view for free. For IMAP profiles: new SQLite table `imap_profiles`, three REST endpoints under `/api/imap/profiles`, and a Profile dropdown + Save / Delete buttons added to the IMAP form.

**Tech Stack:** Go (chi, modernc.org/sqlite, `net/http/httptest`), Vue 3 `<script setup lang=ts>`, vue-router, vue-i18n, Tailwind v4, vue-sonner.

**Verification note:** No frontend test framework exists; per project convention the maintainer runs build/serve. Per-task gates are `go test ./internal/store/... -run IMAPProfiles` / `go test ./internal/server/... -run IMAPProfiles` from the project root, and `npm run type-check` (`vue-tsc --noEmit`) from `web/`. Manual browser checks are listed at the end. Do NOT add a frontend test framework.

---

## File Structure

**Backend (Go):**
- Modify: `internal/store/sqlite.go` — append `imap_profiles` table to `schemaSQL`.
- Create: `internal/store/imap_profiles.go` — `IMAPProfile` struct + `ListIMAPProfiles` / `UpsertIMAPProfile` / `DeleteIMAPProfile` methods + `ErrIMAPProfileNotFound`.
- Create: `internal/store/imap_profiles_test.go` — table-driven CRUD tests against a temp SQLite DB.
- Create: `internal/server/handlers_imap_profiles.go` — three HTTP handlers + validation + `ErrIMAPProfileNotFound` mapping.
- Create: `internal/server/handlers_imap_profiles_test.go` — `httptest.NewRecorder` tests for the 200 / 204 / 400 / 404 paths.
- Modify: `internal/server/router.go` — register the three new routes.

**Frontend (TypeScript / Vue):**
- Modify: `web/src/lib/api.ts` — add `IMAPProfile` interface + `listIMAPProfiles` / `saveIMAPProfile` / `deleteIMAPProfile` methods.
- Modify: `web/src/pages/ImportPage.vue` — Profile row inside the IMAP `<template v-else>` block; mount-time profile load; Save / Delete handlers.
- Modify: `web/src/pages/LibraryPage.vue` — drop `?sel`, drop `EmailDetail` import + right pane, drop `SplitterGroup/SplitterPanel/SplitterResizeHandle` imports + wrapper. Row click navigates instead of selecting.
- Modify: `web/src/components/EmailDetail.vue` — drop `standalone` prop + `popOut()` + pop-out button; always set `document.title` from subject; add Back link.
- Modify: `web/src/pages/ViewerPage.vue` — drop the `standalone` attribute on `<EmailDetail>`.
- Modify: `web/src/router.ts` — remove `chromeless: true` from the viewer route.
- Modify: `web/src/App.vue` — collapse the two layout branches; title `watchEffect` skips the viewer route.
- Modify: `web/src/locales/en.json` and `web/src/locales/ko.json` — restore `viewer.back`; remove `viewer.pop_out` and `library.select_prompt`; add the six new `import.imap_profile*` keys.

---

## Task 1: Backend store — `imap_profiles` table + CRUD

**Files:**
- Modify: `internal/store/sqlite.go`
- Create: `internal/store/imap_profiles.go`
- Create: `internal/store/imap_profiles_test.go`

reka-ui is unaffected. `modernc.org/sqlite` is already imported.

- [ ] **Step 1: Append the `imap_profiles` table to the schema**

In `internal/store/sqlite.go`, inside the `schemaSQL` constant, after the
`CREATE TABLE IF NOT EXISTS import_errors (...)` block, append:

```sql
CREATE TABLE IF NOT EXISTS imap_profiles (
  id INTEGER PRIMARY KEY,
  name TEXT UNIQUE NOT NULL,
  host TEXT NOT NULL,
  port INTEGER,
  username TEXT NOT NULL,
  folder TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
```

Keep the trailing backtick + newline that closes `schemaSQL`. The existing
`migrate` function (single-shot `ExecContext` of the whole string) needs no
change.

- [ ] **Step 2: Create the `IMAPProfile` struct and store methods**

Create `internal/store/imap_profiles.go` with this exact content:

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type IMAPProfile struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Host      string  `json:"host"`
	Port      *int    `json:"port,omitempty"`
	Username  string  `json:"username"`
	Folder    *string `json:"folder,omitempty"`
	CreatedAt int64   `json:"created_at"`
	UpdatedAt int64   `json:"updated_at"`
}

var ErrIMAPProfileNotFound = errors.New("imap profile not found")

func (s *Store) ListIMAPProfiles(ctx context.Context) ([]IMAPProfile, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, name, host, port, username, folder, created_at, updated_at
		FROM imap_profiles
		ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IMAPProfile{}
	for rows.Next() {
		var p IMAPProfile
		var port sql.NullInt64
		var folder sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.Host, &port, &p.Username, &folder, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if port.Valid {
			v := int(port.Int64)
			p.Port = &v
		}
		if folder.Valid {
			v := folder.String
			p.Folder = &v
		}
		out = append(out, p)
	}
	return out, rows.Err()
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

	var existingID int64
	var existingCreated int64
	err = tx.QueryRowContext(ctx,
		`SELECT id, created_at FROM imap_profiles WHERE name = ?`, p.Name,
	).Scan(&existingID, &existingCreated)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := tx.ExecContext(ctx, `
			INSERT INTO imap_profiles (name, host, port, username, folder, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			p.Name, p.Host, portVal, p.Username, folderVal, now, now)
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
			UPDATE imap_profiles SET host = ?, port = ?, username = ?, folder = ?, updated_at = ?
			WHERE id = ?`,
			p.Host, portVal, p.Username, folderVal, now, existingID); err != nil {
			return IMAPProfile{}, err
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
```

- [ ] **Step 3: Write store tests**

Create `internal/store/imap_profiles_test.go`:

```go
package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dsn := "file:" + filepath.Join(dir, "test.db") + "?_pragma=journal_mode(WAL)"
	s, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func ptrInt(v int) *int          { return &v }
func ptrStr(v string) *string    { return &v }

func TestIMAPProfiles_UpsertCreatesAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p, err := s.UpsertIMAPProfile(ctx, IMAPProfile{
		Name: "Work", Host: "imap.example.com", Port: ptrInt(993),
		Username: "user@example.com", Folder: ptrStr("INBOX"),
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if p.ID == 0 || p.CreatedAt == 0 || p.UpdatedAt == 0 {
		t.Fatalf("expected id and timestamps set, got %+v", p)
	}

	list, err := s.ListIMAPProfiles(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Work" || list[0].Host != "imap.example.com" {
		t.Fatalf("unexpected list: %+v", list)
	}
	if list[0].Port == nil || *list[0].Port != 993 {
		t.Fatalf("port not round-tripped: %+v", list[0].Port)
	}
	if list[0].Folder == nil || *list[0].Folder != "INBOX" {
		t.Fatalf("folder not round-tripped: %+v", list[0].Folder)
	}
}

func TestIMAPProfiles_UpsertUpdatesByName(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	first, err := s.UpsertIMAPProfile(ctx, IMAPProfile{
		Name: "Work", Host: "old.example.com", Username: "u",
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := s.UpsertIMAPProfile(ctx, IMAPProfile{
		Name: "Work", Host: "new.example.com", Username: "u2", Folder: ptrStr("Archive"),
	})
	if err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	if updated.ID != first.ID {
		t.Fatalf("id changed: was %d now %d", first.ID, updated.ID)
	}
	if updated.CreatedAt != first.CreatedAt {
		t.Fatalf("created_at changed: was %d now %d", first.CreatedAt, updated.CreatedAt)
	}
	if updated.UpdatedAt < first.UpdatedAt {
		t.Fatalf("updated_at regressed")
	}
	if updated.Host != "new.example.com" || updated.Username != "u2" {
		t.Fatalf("update fields not applied: %+v", updated)
	}

	list, _ := s.ListIMAPProfiles(ctx)
	if len(list) != 1 {
		t.Fatalf("expected 1 row after update, got %d", len(list))
	}
}

func TestIMAPProfiles_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p, _ := s.UpsertIMAPProfile(ctx, IMAPProfile{Name: "X", Host: "h", Username: "u"})
	if err := s.DeleteIMAPProfile(ctx, p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.DeleteIMAPProfile(ctx, p.ID); !errors.Is(err, ErrIMAPProfileNotFound) {
		t.Fatalf("expected ErrIMAPProfileNotFound, got %v", err)
	}
	list, _ := s.ListIMAPProfiles(ctx)
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}
}

func TestIMAPProfiles_NullablePortAndFolder(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.UpsertIMAPProfile(ctx, IMAPProfile{Name: "Y", Host: "h", Username: "u"})
	if err != nil {
		t.Fatal(err)
	}
	list, _ := s.ListIMAPProfiles(ctx)
	if list[0].Port != nil {
		t.Fatalf("expected nil port, got %v", list[0].Port)
	}
	if list[0].Folder != nil {
		t.Fatalf("expected nil folder, got %v", list[0].Folder)
	}
}
```

- [ ] **Step 4: Run the store tests**

From the project root:

```
go test ./internal/store/... -run IMAPProfiles -v
```

Expected: 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/sqlite.go internal/store/imap_profiles.go internal/store/imap_profiles_test.go
git commit -m "$(cat <<'EOF'
feat(store): imap_profiles table + CRUD (password never stored)

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Backend HTTP handlers + router wiring

**Files:**
- Create: `internal/server/handlers_imap_profiles.go`
- Create: `internal/server/handlers_imap_profiles_test.go`
- Modify: `internal/server/router.go`

- [ ] **Step 1: Create the handlers file**

Create `internal/server/handlers_imap_profiles.go`:

```go
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

type imapProfileBody struct {
	Name     string  `json:"name"`
	Host     string  `json:"host"`
	Port     *int    `json:"port,omitempty"`
	Username string  `json:"username"`
	Folder   *string `json:"folder,omitempty"`
}

const (
	maxIMAPProfileName = 64
	minIMAPPort        = 1
	maxIMAPPort        = 65535
)

func (s *Server) handleListIMAPProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := s.Store.ListIMAPProfiles(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, profiles)
}

func (s *Server) handleSaveIMAPProfile(w http.ResponseWriter, r *http.Request) {
	var body imapProfileBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	host := strings.TrimSpace(body.Host)
	username := strings.TrimSpace(body.Username)
	if name == "" || len(name) > maxIMAPProfileName {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}
	if host == "" {
		http.Error(w, "invalid host", http.StatusBadRequest)
		return
	}
	if username == "" {
		http.Error(w, "invalid username", http.StatusBadRequest)
		return
	}
	if body.Port != nil && (*body.Port < minIMAPPort || *body.Port > maxIMAPPort) {
		http.Error(w, "invalid port", http.StatusBadRequest)
		return
	}
	var folder *string
	if body.Folder != nil {
		trimmed := strings.TrimSpace(*body.Folder)
		if trimmed != "" {
			folder = &trimmed
		}
	}
	saved, err := s.Store.UpsertIMAPProfile(r.Context(), store.IMAPProfile{
		Name: name, Host: host, Port: body.Port, Username: username, Folder: folder,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleDeleteIMAPProfile(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.Store.DeleteIMAPProfile(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrIMAPProfileNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Create the handler tests**

Create `internal/server/handlers_imap_profiles_test.go`:

```go
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

func doJSON(t *testing.T, handler http.HandlerFunc, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, rdr)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestIMAPProfilesHandler_SaveListDelete(t *testing.T) {
	s := newTestServer(t)
	r := s.Router()

	// Create
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

	// List
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

	// Delete
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
```

- [ ] **Step 2.5: Add `strconv` to imports if not already there**

The test file uses `strconv`; it's already in the import block above. No
adjustment needed.

- [ ] **Step 3: Wire up the routes in `router.go`**

In `internal/server/router.go`, inside `r.Route("/api", func(api chi.Router) { ... })`, after the existing tag routes, add:

```go
		api.Get("/imap/profiles", s.handleListIMAPProfiles)
		api.Post("/imap/profiles", s.handleSaveIMAPProfile)
		api.Delete("/imap/profiles/{id}", s.handleDeleteIMAPProfile)
```

(Indent with tabs to match the existing routes.)

- [ ] **Step 4: Run the handler tests**

From the project root:

```
go test ./internal/server/... -run IMAPProfilesHandler -v
```

Expected: 4 tests PASS (including the 6 sub-tests under `TestIMAPProfilesHandler_Validation`).

- [ ] **Step 5: Run the full backend test suite to confirm nothing else broke**

```
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/server/handlers_imap_profiles.go internal/server/handlers_imap_profiles_test.go internal/server/router.go
git commit -m "$(cat <<'EOF'
feat(server): GET/POST/DELETE /api/imap/profiles

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Frontend API client

**Files:**
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add the `IMAPProfile` interface and three methods**

In `web/src/lib/api.ts`, immediately after the existing `IMAPImportConfig`
interface (currently lines 79-85), add:

```ts
export interface IMAPProfile {
  id: number
  name: string
  host: string
  port?: number
  username: string
  folder?: string
}
```

Then inside the `export const api = {` object (currently ends at line 183),
add three methods just before the closing brace, after the `importEventSource`
method:

```ts
  listIMAPProfiles() {
    return fetch(`${BASE}/api/imap/profiles`).then(jsonOrThrow<IMAPProfile[]>)
  },

  async saveIMAPProfile(p: Omit<IMAPProfile, 'id'>): Promise<IMAPProfile> {
    const res = await fetch(`${BASE}/api/imap/profiles`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(p),
    })
    return jsonOrThrow<IMAPProfile>(res)
  },

  async deleteIMAPProfile(id: number): Promise<void> {
    const res = await fetch(`${BASE}/api/imap/profiles/${id}`, { method: 'DELETE' })
    if (!res.ok) {
      const text = await res.text().catch(() => '')
      throw new Error(`${res.status} ${res.statusText}: ${text}`)
    }
  },
```

Note: `jsonOrThrow` is the existing helper at the top of the file (line 89);
its generic-typed call form (`jsonOrThrow<X>`) matches the surrounding
`importStatus` / `listTags` callers.

- [ ] **Step 2: Type-check**

From `web/`:

```
npm run type-check
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/api.ts
git commit -m "$(cat <<'EOF'
feat(web): api client for /api/imap/profiles

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Frontend IMAP profile UI in `ImportPage.vue` + i18n

**Files:**
- Modify: `web/src/pages/ImportPage.vue`
- Modify: `web/src/locales/en.json`
- Modify: `web/src/locales/ko.json`

- [ ] **Step 1: Add the six new i18n keys**

In `web/src/locales/en.json`, inside the `"import": { ... }` block (currently
between lines 58-103), add the following six lines just before the closing
`}` of the import block — anywhere after `imap_confirm_hint` line is fine,
keep them grouped:

```json
    "imap_profile": "Profile",
    "imap_profile_new": "— New —",
    "imap_profile_save": "Save",
    "imap_profile_save_prompt": "Profile name:",
    "imap_profile_delete": "Delete",
    "imap_profile_delete_confirm": "Delete profile \"{name}\"?",
    "imap_profile_save_error": "Couldn't save profile",
    "imap_profile_delete_error": "Couldn't delete profile"
```

Make sure to add a trailing comma to whichever line precedes them so the JSON
remains valid.

In `web/src/locales/ko.json`, add the matching keys in the same location:

```json
    "imap_profile": "프로필",
    "imap_profile_new": "— 새 프로필 —",
    "imap_profile_save": "저장",
    "imap_profile_save_prompt": "프로필 이름:",
    "imap_profile_delete": "삭제",
    "imap_profile_delete_confirm": "\"{name}\" 프로필을 삭제할까요?",
    "imap_profile_save_error": "프로필을 저장하지 못했어요",
    "imap_profile_delete_error": "프로필을 삭제하지 못했어요"
```

- [ ] **Step 2: Add the profile state and handlers to `ImportPage.vue`**

In `web/src/pages/ImportPage.vue`, modify the `<script setup>` block:

(a) Update the import from `@/lib/api` to include `IMAPProfile` (currently
line 7 imports `S3ImportConfig, IMAPImportConfig`):

```ts
import type { S3ImportConfig, IMAPImportConfig, IMAPProfile } from '@/lib/api'
```

(b) Add `onMounted` and `toast` to the existing imports. Update the Vue import
(currently `import { ref, computed } from 'vue'`) to:

```ts
import { ref, computed, onMounted } from 'vue'
```

(c) Add the toast import after the existing `import { useImports }` line:

```ts
import { toast } from 'vue-sonner'
import { api } from '@/lib/api'
```

(d) Just before the line `// --- IMAP provider ---` (currently around line 148),
insert the profile state and handlers:

```ts
// --- IMAP profiles ---
const imapProfiles = ref<IMAPProfile[]>([])
const selectedImapProfileId = ref<number | null>(null)

const activeImapProfile = computed(() =>
  imapProfiles.value.find((p) => p.id === selectedImapProfileId.value) ?? null,
)

async function loadImapProfiles() {
  try {
    imapProfiles.value = await api.listIMAPProfiles()
  } catch (e) {
    toast.error(t('import.imap_profile_save_error'), { description: String(e) })
  }
}

onMounted(() => {
  loadImapProfiles()
})

function applyImapProfile(p: IMAPProfile | null) {
  if (!p) {
    imapForm.value = { host: '', port: 993, username: '', password: '', folder: 'INBOX' }
    return
  }
  imapForm.value = {
    host: p.host,
    port: p.port ?? 993,
    username: p.username,
    password: '',
    folder: p.folder ?? 'INBOX',
  }
}

function onImapProfileChange(idStr: string) {
  const id = idStr ? Number(idStr) : null
  selectedImapProfileId.value = id
  applyImapProfile(id == null ? null : imapProfiles.value.find((p) => p.id === id) ?? null)
}

async function saveImapProfile() {
  const existing = activeImapProfile.value
  let name = existing?.name ?? ''
  if (!existing) {
    const entered = window.prompt(t('import.imap_profile_save_prompt'), '')
    if (entered == null) return
    name = entered.trim()
    if (!name) return
  }
  try {
    const saved = await api.saveIMAPProfile({
      name,
      host: imapForm.value.host.trim(),
      port: imapForm.value.port || undefined,
      username: imapForm.value.username.trim(),
      folder: imapForm.value.folder?.trim() || undefined,
    })
    await loadImapProfiles()
    selectedImapProfileId.value = saved.id
  } catch (e) {
    toast.error(t('import.imap_profile_save_error'), { description: String(e) })
  }
}

async function deleteImapProfile() {
  const p = activeImapProfile.value
  if (!p) return
  if (!window.confirm(t('import.imap_profile_delete_confirm', { name: p.name }))) return
  try {
    await api.deleteIMAPProfile(p.id)
    selectedImapProfileId.value = null
    applyImapProfile(null)
    await loadImapProfiles()
  } catch (e) {
    toast.error(t('import.imap_profile_delete_error'), { description: String(e) })
  }
}
```

(e) The Save button should be enabled only when host/username are non-empty;
add this computed near the existing `imapValid`:

```ts
const canSaveImapProfile = computed(
  () =>
    imapForm.value.host.trim().length > 0 &&
    imapForm.value.username.trim().length > 0,
)
```

- [ ] **Step 3: Add the profile row to the IMAP form template**

In the template, inside the `<Card v-if="!imapConfirming" class="p-6 space-y-4">`
block (currently starting around line 330), immediately after the
`<div><h3>{{ t('import.imap_title') }}</h3>...</div>` heading block and BEFORE
the `<div class="grid gap-4 sm:grid-cols-2">` host/port grid, add:

```vue
<div class="flex items-end gap-2">
  <label class="space-y-1 flex-1 max-w-xs">
    <span class="text-sm">{{ t('import.imap_profile') }}</span>
    <select
      :value="selectedImapProfileId ?? ''"
      @change="onImapProfileChange(($event.target as HTMLSelectElement).value)"
      :class="inputClass"
    >
      <option value="">{{ t('import.imap_profile_new') }}</option>
      <option v-for="p in imapProfiles" :key="p.id" :value="p.id">{{ p.name }}</option>
    </select>
  </label>
  <Button variant="outline" :disabled="!canSaveImapProfile" @click="saveImapProfile">
    {{ t('import.imap_profile_save') }}
  </Button>
  <Button variant="outline" :disabled="!activeImapProfile" @click="deleteImapProfile">
    {{ t('import.imap_profile_delete') }}
  </Button>
</div>
```

- [ ] **Step 4: Type-check**

From `web/`:

```
npm run type-check
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/ImportPage.vue web/src/locales/en.json web/src/locales/ko.json
git commit -m "$(cat <<'EOF'
feat(web): named IMAP profile dropdown with save/delete (password never persisted)

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Revert split-panel layout in `LibraryPage.vue`

**Files:**
- Modify: `web/src/pages/LibraryPage.vue`

- [ ] **Step 1: Update the `<script setup>` block**

In `web/src/pages/LibraryPage.vue`:

(a) Remove these imports (currently lines 12-13):

```ts
import EmailDetail from '@/components/EmailDetail.vue'
import { SplitterGroup, SplitterPanel, SplitterResizeHandle } from 'reka-ui'
```

(b) Remove the `Card` import only if it's not used anywhere else in the file —
search the file: if `Card` does not appear anywhere outside the deleted detail
pane, also delete `import Card from '@/components/ui/Card.vue'`. Otherwise
leave it.

(c) Remove the `selected` computed (currently line 48):

```ts
const selected = computed(() => str(route.query.sel))
```

(d) Replace the `select` function (currently lines 124-126):

```ts
function select(sha: string) {
  replaceQuery({ sel: sha === selected.value ? undefined : sha })
}
```

with:

```ts
function openEmail(sha: string) {
  router.push({ name: 'viewer', params: { sha } })
}
```

- [ ] **Step 2: Update the `<template>` block**

In the template:

(a) The current row binding is:

```vue
<tr
  v-for="e in items"
  :key="e.sha256"
  class="border-t hover:bg-accent/50 cursor-pointer"
  :class="{ 'bg-accent': e.sha256 === selected }"
  @click="select(e.sha256)"
>
```

Change to (drop the selection-highlight class and rename the handler):

```vue
<tr
  v-for="e in items"
  :key="e.sha256"
  class="border-t hover:bg-accent/50 cursor-pointer"
  @click="openEmail(e.sha256)"
>
```

(b) Replace the entire SplitterGroup wrapper. The current template (starting
around line 175):

```vue
    <SplitterGroup direction="horizontal" auto-save-id="library-split" class="flex-1 min-w-0">
      <SplitterPanel :default-size="58" :min-size="30" class="min-w-0">
        <section class="min-w-0">
          <div class="flex items-center gap-3 mb-4">
            ...
          </div>

          <Card class="overflow-hidden">
            ...
          </Card>

          <div class="flex justify-between items-center mt-4">
            ...
          </div>
        </section>
      </SplitterPanel>

      <SplitterResizeHandle
        class="w-1.5 mx-1 shrink-0 rounded-full bg-muted hover:bg-accent focus-visible:bg-accent focus:outline-none cursor-col-resize transition-colors"
      />

      <SplitterPanel :default-size="42" :min-size="25" class="min-w-0">
        <aside class="min-w-0">
          <EmailDetail v-if="selected" :sha="selected" />
          <Card v-else class="p-10 text-center text-muted-foreground">
            {{ t('library.select_prompt') }}
          </Card>
        </aside>
      </SplitterPanel>
    </SplitterGroup>
```

becomes (a plain `<section>` with no splitter, no detail pane):

```vue
    <section class="flex-1 min-w-0">
      <div class="flex items-center gap-3 mb-4">
        <Input v-model="searchInput" :placeholder="t('library.search_placeholder')" class="max-w-md" />
        <span class="text-sm text-muted-foreground ml-auto">{{ pageInfo }}</span>
      </div>

      <Card class="overflow-hidden">
        <table class="w-full text-sm">
          <thead class="bg-muted/40 text-xs uppercase text-muted-foreground">
            <tr>
              <th class="text-left px-3 py-2 w-10"></th>
              <Th :label="t('library.col.date')" col="sent_at" :sort="sort" :order="order" @sort="setSort" />
              <Th :label="t('library.col.from')" col="from_addr" :sort="sort" :order="order" @sort="setSort" />
              <Th :label="t('library.col.subject')" col="subject" :sort="sort" :order="order" @sort="setSort" />
              <Th :label="t('library.col.size')" col="size_bytes" :sort="sort" :order="order" @sort="setSort" align="right" />
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading && items.length === 0">
              <td colspan="5" class="px-3 py-6 text-center text-muted-foreground">{{ t('library.loading') }}</td>
            </tr>
            <tr v-else-if="items.length === 0">
              <td colspan="5" class="px-3 py-6 text-center text-muted-foreground">
                {{ t('library.no_emails') }}
                <RouterLink to="/import" class="underline">{{ t('library.import_some') }}</RouterLink>
              </td>
            </tr>
            <tr
              v-for="e in items"
              :key="e.sha256"
              class="border-t hover:bg-accent/50 cursor-pointer"
              @click="openEmail(e.sha256)"
            >
              <td class="px-3 py-2 text-muted-foreground">
                <span v-if="e.has_attachments" :title="t('library.has_attachments')">📎</span>
              </td>
              <td class="px-3 py-2 whitespace-nowrap text-muted-foreground">{{ formatDate(e.sent_at) }}</td>
              <td class="px-3 py-2 truncate max-w-[18rem]" :title="e.from">{{ e.from }}</td>
              <td class="px-3 py-2 truncate">
                {{ e.subject || t('library.no_subject') }}
                <span v-for="t2 in e.tags" :key="t2" class="ml-1 inline-block text-[10px] px-1.5 py-0.5 rounded bg-accent text-accent-foreground">{{ t2 }}</span>
                <span class="ml-2 text-xs text-muted-foreground">{{ shortSHA(e.sha256) }}</span>
              </td>
              <td class="px-3 py-2 text-right whitespace-nowrap text-muted-foreground">{{ formatBytes(e.size_bytes) }}</td>
            </tr>
          </tbody>
        </table>
      </Card>

      <div class="flex justify-between items-center mt-4">
        <Button variant="outline" size="sm" :disabled="offset === 0" @click="setOffset(Math.max(0, offset - limit))">{{ t('library.prev') }}</Button>
        <span class="text-sm text-muted-foreground">{{ pageInfo }}</span>
        <Button variant="outline" size="sm" :disabled="offset + limit >= total" @click="setOffset(offset + limit)">{{ t('library.next') }}</Button>
      </div>
    </section>
```

The tags `Sidebar` block and the collapsed-toggle `<button v-else …>` above
it stay exactly as they are. The trailing `<script lang="ts">` `Th` helper
block at the bottom of the file is unchanged.

- [ ] **Step 3: Type-check**

From `web/`:

```
npm run type-check
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/LibraryPage.vue
git commit -m "$(cat <<'EOF'
revert(web): drop split panel; library row click navigates to /email/:sha

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Drop `standalone` from `EmailDetail.vue`; add Back button

**Files:**
- Modify: `web/src/components/EmailDetail.vue`
- Modify: `web/src/locales/en.json`
- Modify: `web/src/locales/ko.json`

- [ ] **Step 1: Add the `viewer.back` i18n key**

In `web/src/locales/en.json`, inside the `"viewer"` block (currently around
lines 32-57), replace the `"pop_out": "⧉ Open in new window",` line with:

```json
    "back": "← Back to library",
```

In `web/src/locales/ko.json`, the matching block, replace
`"pop_out": "⧉ 새 창에서 열기",` with:

```json
    "back": "← 라이브러리로 돌아가기",
```

(The `viewer.pop_out` key is now gone; the new `viewer.back` key replaces it
in the same alphabetical slot — that's intentional.)

- [ ] **Step 2: Simplify `EmailDetail.vue`**

In `web/src/components/EmailDetail.vue`:

(a) Change the props line (line 18) from:

```ts
const props = defineProps<{ sha: string; standalone?: boolean }>()
```

to:

```ts
const props = defineProps<{ sha: string }>()
```

(b) Add `useRouter` to the existing `vue-router`-related imports. After the
existing `import { useI18n } from 'vue-i18n'` line, add:

```ts
import { useRouter } from 'vue-router'
```

(c) Below the existing `const { t } = useI18n()` line, add:

```ts
const router = useRouter()
```

(d) Replace the existing title watcher (currently lines 63-65):

```ts
watch([() => email.value?.subject, () => props.standalone], ([subject, standalone]) => {
  if (standalone) document.title = subject ? `${subject} — ${APP_NAME}` : APP_NAME
})
```

with the unconditional version:

```ts
watch(() => email.value?.subject, (subject) => {
  document.title = subject ? `${subject} — ${APP_NAME}` : APP_NAME
})
```

(e) Replace the `popOut` function (currently lines 67-69):

```ts
function popOut() {
  window.open(`/email/${props.sha}`, '_blank', 'popup,width=820,height=900')
}
```

with:

```ts
function goBack() {
  if (window.history.length > 1) router.back()
  else router.push('/')
}
```

(f) In the template, replace the `<button v-if="!standalone" …>` pop-out
button (currently lines 102-109):

```vue
<button
  v-if="!standalone"
  type="button"
  @click="popOut"
  class="shrink-0 text-xs text-muted-foreground hover:text-foreground rounded-sm px-2 py-1 hover:bg-accent"
>
  {{ t('viewer.pop_out') }}
</button>
```

with the back button placed at the very top of the email card. Specifically:
delete the pop-out `<button>` element entirely (the surrounding `<div class="flex items-start gap-3 mb-3">` now contains only `<h1>`, which can drop its `flex-1 min-w-0` since the row no longer needs to stretch). Then add a back control **above** the first `<Card class="p-5">` in the `<div v-else-if="email" class="space-y-4">` block. The result around that area should look like:

```vue
<div v-else-if="email" class="space-y-4">
  <button
    type="button"
    @click="goBack"
    class="inline-flex items-center text-sm text-muted-foreground hover:text-foreground"
  >
    {{ t('viewer.back') }}
  </button>

  <Card class="p-5">
    <div class="mb-3">
      <h1 class="text-xl font-semibold">{{ email.subject || t('library.no_subject') }}</h1>
    </div>
    <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
      ...unchanged...
```

(The rest of the template — tabs, iframe, attachments, etc. — is unchanged.)

- [ ] **Step 3: Type-check**

From `web/`:

```
npm run type-check
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/EmailDetail.vue web/src/locales/en.json web/src/locales/ko.json
git commit -m "$(cat <<'EOF'
refactor(web): EmailDetail drops standalone+popout; adds Back to library

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Drop chrome-less mode (`ViewerPage`, `router.ts`, `App.vue`) + `library.select_prompt`

**Files:**
- Modify: `web/src/pages/ViewerPage.vue`
- Modify: `web/src/router.ts`
- Modify: `web/src/App.vue`
- Modify: `web/src/locales/en.json`
- Modify: `web/src/locales/ko.json`

- [ ] **Step 1: Remove `standalone` from `ViewerPage.vue`**

Replace the entire body of `web/src/pages/ViewerPage.vue` with:

```vue
<script setup lang="ts">
import EmailDetail from '@/components/EmailDetail.vue'

defineProps<{ sha: string }>()
</script>

<template>
  <EmailDetail :sha="sha" />
</template>
```

- [ ] **Step 2: Drop `chromeless` from the viewer route in `router.ts`**

Replace the current viewer line (line 5):

```ts
{ path: '/email/:sha', name: 'viewer', component: () => import('@/pages/ViewerPage.vue'), props: true, meta: { titleKey: 'nav.viewer', chromeless: true } },
```

with:

```ts
{ path: '/email/:sha', name: 'viewer', component: () => import('@/pages/ViewerPage.vue'), props: true, meta: { titleKey: 'nav.viewer' } },
```

- [ ] **Step 3: Collapse the two layout branches in `App.vue`**

Replace the entire content of `web/src/App.vue` with:

```vue
<script setup lang="ts">
import { computed, watchEffect } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Toaster } from 'vue-sonner'
import 'vue-sonner/style.css'
import { APP_NAME } from '@/lib/app'

const { t } = useI18n()
const route = useRoute()

const nav = computed(() => [
  { to: '/', label: t('nav.library') },
  { to: '/import', label: t('nav.import') },
  { to: '/settings', label: t('nav.settings') },
])

const pageTitle = computed(() => {
  const key = route.meta.titleKey as string | undefined
  return key ? t(key) : ''
})

watchEffect(() => {
  // EmailDetail owns the title on the viewer route (it writes the subject).
  if (route.name === 'viewer') return
  document.title = pageTitle.value ? `${APP_NAME} | ${pageTitle.value}` : APP_NAME
})
</script>

<template>
  <div class="min-h-screen flex flex-col">
    <header class="bg-black text-white">
      <div class="max-w-[1800px] mx-auto px-6 h-11 flex items-center gap-6 text-xs">
        <RouterLink to="/" class="flex items-center gap-2 font-semibold tracking-tight text-white/90 hover:text-white">
          <img src="/icon-64.png" srcset="/favicon.png 1x, /icon-64.png 2x" alt="" class="h-6 w-6 rounded-sm" />
          <span>{{ APP_NAME }}</span>
        </RouterLink>
        <nav class="flex items-center gap-1">
          <RouterLink
            v-for="r in nav"
            :key="r.to"
            :to="r.to"
            class="px-3 py-1 rounded-xs text-white/70 hover:text-white"
            active-class="text-white"
          >
            {{ r.label }}
          </RouterLink>
        </nav>
      </div>
    </header>
    <main class="flex-1 max-w-[1800px] mx-auto w-full px-6 py-10">
      <RouterView />
    </main>
  </div>

  <Toaster position="bottom-right" rich-colors close-button />
</template>
```

- [ ] **Step 4: Remove the now-unused `library.select_prompt` key from i18n**

In `web/src/locales/en.json`, inside the `"library"` block, delete the line:

```json
    "select_prompt": "Select an email to read.",
```

In `web/src/locales/ko.json`, delete:

```json
    "select_prompt": "읽을 이메일을 선택하세요.",
```

Both files should remain valid JSON (preceding line's trailing comma must be
preserved — `select_prompt` is currently followed by `search_placeholder`, so
removing the `select_prompt` line is straightforward).

- [ ] **Step 5: Type-check**

From `web/`:

```
npm run type-check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/ViewerPage.vue web/src/router.ts web/src/App.vue web/src/locales/en.json web/src/locales/ko.json
git commit -m "$(cat <<'EOF'
refactor(web): drop chrome-less viewer mode; viewer route uses normal app layout

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Manual Verification (maintainer)

After all seven tasks land, run `make web-dev` (Vite dev server) or
`make build && ./local-eml serve`, then:

**Layout revert (Tasks 5–7):**

1. Open `/` → click a row → URL is `/email/:sha`, app header + nav are
   visible, the Back link is at the top of the email card.
2. Click Back → returns to `/` with whatever filter/sort/page query you had
   before (because the click was a `router.push`).
3. Apply `?q=foo&sort=subject&order=asc&offset=50` directly in the URL,
   click a row, then Back → that query is intact.
4. Open `/email/:sha` directly (deep link, no prior history) → Back link goes
   to `/` (empty query); no out-of-app navigation.
5. No pop-out button is visible on the detail page; chrome-less mode is gone.

**IMAP profiles (Tasks 1–4):**

6. Open `/import` → IMAP tab. The "Profile" dropdown shows just `— New —`.
7. Fill host/port/username/folder/password → click **Save** → prompt asks for
   a name → enter `Work` → toast-free success, the dropdown now shows `Work`
   selected.
8. Reload the page → dropdown auto-loads `Work`; selecting it fills host /
   port / username / folder; password field is empty.
9. Change `folder` to `Archive` and click **Save** (no prompt this time) →
   reload → folder change persisted.
10. Click **Delete** → confirm → entry disappears from the dropdown; form
    resets to the `— New —` defaults.
11. Open SQLite (`sqlite3 ~/.local-eml/db/local-eml.db ".schema imap_profiles"`)
    → confirm there is no `password` column.

Backend: `go test ./...` is green (includes the new store + handler tests).

---

## Self-Review

**Spec coverage:**

- Spec §"Change 1 — Detail returns to its own page" → Tasks 5 (LibraryPage),
  6 (EmailDetail + viewer.back), 7 (ViewerPage / router / App / drop
  pop_out & select_prompt).
- Spec §"Change 2 — Named IMAP profiles" data model → Task 1.
- Spec §"Change 2" store layer → Task 1.
- Spec §"Change 2" server endpoints → Task 2.
- Spec §"Change 2" frontend API client → Task 3.
- Spec §"Change 2" UI + i18n → Task 4.
- Spec §"History semantics" → Task 5's `router.push` + Task 6's `goBack`
  with `router.back()` / fallback.
- Spec §"Security notes" — Tasks 1 (no password column) + 4 (password never
  POSTed to /profiles, only to the existing /imports/imap endpoint).

**Placeholder scan:** none. Every code block contains the exact text to
write. The phrase "search the file: if `Card` does not appear anywhere
outside the deleted detail pane" in Task 5 is a conditional decision an
implementer can make in <30 seconds, not a placeholder for missing content.

**Type / name consistency:**

- `IMAPProfile` shape (id, name, host, port?, username, folder?) is identical
  in Task 1 (Go struct + JSON tags), Task 3 (TS interface), Task 4 (UI usage).
- `ErrIMAPProfileNotFound` defined in Task 1, consumed in Task 2.
- `s.Store.ListIMAPProfiles` / `UpsertIMAPProfile` / `DeleteIMAPProfile`
  signatures match between Task 1 (definition) and Task 2 (callers).
- Handler names `handleListIMAPProfiles` / `handleSaveIMAPProfile` /
  `handleDeleteIMAPProfile` defined in Task 2 and wired in Task 2 step 3.
- Frontend `api.listIMAPProfiles` / `saveIMAPProfile` / `deleteIMAPProfile`
  defined in Task 3, consumed in Task 4.
- New i18n keys (`import.imap_profile`, `import.imap_profile_new`,
  `import.imap_profile_save`, `import.imap_profile_save_prompt`,
  `import.imap_profile_delete`, `import.imap_profile_delete_confirm`,
  `import.imap_profile_save_error`, `import.imap_profile_delete_error`)
  added in Task 4 match the `t(…)` calls in the same task.
- `viewer.back` key added in Task 6 matches the `t('viewer.back')` call in
  the same task.
- `library.select_prompt` removed in Task 7 — verified that Task 5 also
  removes its only reference (the placeholder `<Card>` in the deleted right
  pane); no other file references it.
- `viewer.pop_out` removed in Task 6 — verified that the only reference was
  in `EmailDetail.vue`'s pop-out button, also removed in Task 6.
