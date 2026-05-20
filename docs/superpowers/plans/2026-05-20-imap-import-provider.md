# IMAP Import Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an IMAP import provider that connects read-only to a mail server, lists every message in one folder (default INBOX), and imports each raw RFC822 message through the existing dedup/parse/store pipeline — plugged into the existing `Source`/`SourceCloser` seam.

**Architecture:** A new `imapSource` implements `SourceCloser`. `Scan` dials/logs-in/`EXAMINE`s the folder and `UID SEARCH ALL` for a UID list; each `Item.Open(uid)` does `UID FETCH BODY.PEEK[]`, buffering one message at a time. The go-imap client is isolated behind a small `imapSession` interface so the source logic is unit-tested with a fake; the real dial/login adapter is thin and manually smoke-tested. A JSON endpoint `POST /api/imports/imap` and a third frontend provider tab complete it.

**Tech Stack:** Go 1.25, `github.com/emersion/go-imap/v2` (v2.0.0-beta.8); chi, SSE; Vue 3 + Vite + TypeScript + Tailwind, vue-i18n.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/importer/source_imap.go` (create) | `IMAPConfig`, `imapSession` interface, `imapSource` (SourceCloser), real `imapClientSession` adapter, `NewIMAPSource` |
| `internal/importer/source_imap_test.go` (create) | unit tests with a fake `imapSession` + a `RunSource` integration test |
| `internal/server/handlers_imports.go` (modify) | add `handleImportImap` |
| `internal/server/router.go` (modify) | register `POST /api/imports/imap` |
| `web/src/lib/api.ts` (modify) | `IMAPImportConfig` + `uploadImap` |
| `web/src/composables/useImports.ts` (modify) | `startImapImport` |
| `web/src/pages/ImportPage.vue` (modify) | third provider tab + IMAP form |
| `web/src/locales/{en,ko}.json` (modify) | new `import.*` IMAP keys |

**Commands used throughout:**
- Go build: `CGO_ENABLED=0 go build ./...`
- Go tests: `CGO_ENABLED=1 go test ./internal/importer/... -race -count=1`
- Web type-check: `cd web && npm run type-check`

The interactive shell has alias wrappers that hijack bare `curl`/`cat`; `go`/`git`/`npm` work normally. Do NOT stage `web/package-lock.json` (pre-existing unrelated change).

---

## Task 1: Add go-imap/v2 dependency

**Files:** Modify `go.mod`, `go.sum`

- [ ] **Step 1: Add the module**

Run:
```bash
go get github.com/emersion/go-imap/v2@v2.0.0-beta.8
```
(Version confirmed to exist on proxy.golang.org. It will be `// indirect` until Task 2 imports it — that is expected; Task 2 runs `go mod tidy` to promote it.)

- [ ] **Step 2: Verify build**

Run: `CGO_ENABLED=0 go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "build: add emersion/go-imap/v2"
```

---

## Task 2: IMAP source

**Files:**
- Create: `internal/importer/source_imap.go`
- Test: `internal/importer/source_imap_test.go`

Context for the implementer: the package already has `Source`/`Item`/`SourceCloser` (`source.go`), the `RunSource` driver (`job.go`), and test helpers `newTestStore(t)`, `newTestPaths(t)`, and the const `minimalEML` (all in `source_test.go`, same package — reuse them, do NOT redefine).

- [ ] **Step 1: Write the failing unit + integration tests**

Create `internal/importer/source_imap_test.go`:
```go
package importer

import (
	"context"
	"errors"
	"io"
	"testing"

	imap "github.com/emersion/go-imap/v2"
)

type fakeIMAP struct {
	uids     []imap.UID
	bodies   map[imap.UID]string
	fetchErr map[imap.UID]error
	closed   bool
}

func (f *fakeIMAP) UIDs() ([]imap.UID, error) { return f.uids, nil }

func (f *fakeIMAP) Fetch(uid imap.UID) ([]byte, error) {
	if f.fetchErr != nil {
		if err := f.fetchErr[uid]; err != nil {
			return nil, err
		}
	}
	return []byte(f.bodies[uid]), nil
}

func (f *fakeIMAP) Close() error { f.closed = true; return nil }

func sourceWithFake(f *fakeIMAP, cfg IMAPConfig) *imapSource {
	return &imapSource{cfg: cfg, dial: func(IMAPConfig) (imapSession, error) { return f, nil }}
}

func TestIMAPSourceScanMapsUIDsToItems(t *testing.T) {
	f := &fakeIMAP{
		uids:   []imap.UID{7, 9},
		bodies: map[imap.UID]string{7: "SEVEN", 9: "NINE"},
	}
	src := sourceWithFake(f, IMAPConfig{Host: "h", Username: "u"})
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	rc, err := items[0].Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, _ := io.ReadAll(rc)
	if string(b) != "SEVEN" {
		t.Errorf("body = %q, want SEVEN", string(b))
	}
}

func TestIMAPSourceFetchErrorSurfaces(t *testing.T) {
	f := &fakeIMAP{
		uids:     []imap.UID{1},
		fetchErr: map[imap.UID]error{1: errors.New("boom")},
	}
	src := sourceWithFake(f, IMAPConfig{Host: "h", Username: "u"})
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := items[0].Open(context.Background()); err == nil {
		t.Error("expected Open error, got nil")
	}
}

func TestIMAPSourceLabel(t *testing.T) {
	def := &imapSource{cfg: IMAPConfig{Host: "mail.example.com", Username: "alice"}}
	if got := def.Label(); got != "imap://alice@mail.example.com/INBOX" {
		t.Errorf("label = %q", got)
	}
	named := &imapSource{cfg: IMAPConfig{Host: "h", Username: "u", Folder: "Archive"}}
	if got := named.Label(); got != "imap://u@h/Archive" {
		t.Errorf("label = %q", got)
	}
}

func TestIMAPSourceCloseClosesSession(t *testing.T) {
	f := &fakeIMAP{uids: []imap.UID{1}, bodies: map[imap.UID]string{1: "x"}}
	src := sourceWithFake(f, IMAPConfig{Host: "h", Username: "u"})
	if _, err := src.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}
	if !f.closed {
		t.Error("Close did not close the session")
	}
}

func TestRunSourceDrivesIMAPSource(t *testing.T) {
	st := newTestStore(t)
	im := &Importer{Store: st, Paths: newTestPaths(t)}
	hub := NewHub()

	importID := "imap1"
	if err := st.CreateImport(context.Background(), importStub(importID)); err != nil {
		t.Fatal(err)
	}

	f := &fakeIMAP{
		uids:     []imap.UID{1, 2},
		bodies:   map[imap.UID]string{1: minimalEML},
		fetchErr: map[imap.UID]error{2: errors.New("nope")},
	}
	src := sourceWithFake(f, IMAPConfig{Host: "h", Username: "u"})

	job := &Job{Importer: im, Hub: hub, Store: st, ID: importID}
	job.RunSource(context.Background(), src)

	imp, err := st.GetImport(context.Background(), importID)
	if err != nil {
		t.Fatal(err)
	}
	if imp.Status != "done" {
		t.Errorf("status = %q, want done", imp.Status)
	}
	if imp.Total != 2 || imp.Processed != 2 || imp.Errors != 1 {
		t.Errorf("counters total=%d processed=%d errors=%d, want 2/2/1",
			imp.Total, imp.Processed, imp.Errors)
	}
}
```

- [ ] **Step 2: Run tests, confirm they FAIL**

Run: `CGO_ENABLED=1 go test ./internal/importer/... -run IMAP -count=1`
Expected: FAIL — `undefined: imapSource` / `IMAPConfig` / `imapSession`.

- [ ] **Step 3: Implement the source + adapter**

Create `internal/importer/source_imap.go`:
```go
package importer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

type IMAPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Folder   string
}

// imapSession isolates the go-imap client so imapSource is unit-testable with a fake.
type imapSession interface {
	UIDs() ([]imap.UID, error)
	Fetch(uid imap.UID) ([]byte, error)
	Close() error
}

type imapSource struct {
	cfg     IMAPConfig
	dial    func(IMAPConfig) (imapSession, error)
	session imapSession
}

func NewIMAPSource(cfg IMAPConfig) SourceCloser {
	return &imapSource{cfg: cfg, dial: newIMAPSession}
}

func (s *imapSource) folder() string {
	if s.cfg.Folder == "" {
		return "INBOX"
	}
	return s.cfg.Folder
}

func (s *imapSource) Label() string {
	return fmt.Sprintf("imap://%s@%s/%s", s.cfg.Username, s.cfg.Host, s.folder())
}

func (s *imapSource) Scan(_ context.Context) ([]Item, error) {
	sess, err := s.dial(s.cfg)
	if err != nil {
		return nil, err
	}
	s.session = sess

	uids, err := sess.UIDs()
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(uids))
	for _, uid := range uids {
		u := uid
		items = append(items, Item{
			Name: fmt.Sprintf("uid-%d.eml", u),
			Open: func(context.Context) (io.ReadCloser, error) {
				b, err := sess.Fetch(u)
				if err != nil {
					return nil, err
				}
				return io.NopCloser(bytes.NewReader(b)), nil
			},
		})
	}
	return items, nil
}

func (s *imapSource) Close() error {
	if s.session != nil {
		return s.session.Close()
	}
	return nil
}

// imapClientSession is the real adapter over go-imap/v2. It is NOT unit-tested
// (the maintainer smoke-tests it against a real server); it sits behind imapSession.
type imapClientSession struct {
	client      *imapclient.Client
	bodySection *imap.FetchItemBodySection
}

func newIMAPSession(cfg IMAPConfig) (imapSession, error) {
	port := cfg.Port
	if port == 0 {
		port = 993
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(port))

	var (
		c   *imapclient.Client
		err error
	)
	if port == 143 {
		c, err = imapclient.DialStartTLS(addr, nil)
	} else {
		c, err = imapclient.DialTLS(addr, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("imap dial: %w", err)
	}

	if err := c.Login(cfg.Username, cfg.Password).Wait(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("imap login: %w", err)
	}

	folder := cfg.Folder
	if folder == "" {
		folder = "INBOX"
	}
	if _, err := c.Select(folder, &imap.SelectOptions{ReadOnly: true}).Wait(); err != nil {
		_ = c.Logout().Wait()
		_ = c.Close()
		return nil, fmt.Errorf("imap select %q: %w", folder, err)
	}

	return &imapClientSession{
		client:      c,
		bodySection: &imap.FetchItemBodySection{Specifier: imap.PartSpecifierFull, Peek: true},
	}, nil
}

func (s *imapClientSession) UIDs() ([]imap.UID, error) {
	data, err := s.client.UIDSearch(&imap.SearchCriteria{}, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("imap search: %w", err)
	}
	return data.AllUIDs(), nil
}

func (s *imapClientSession) Fetch(uid imap.UID) ([]byte, error) {
	opts := &imap.FetchOptions{BodySection: []*imap.FetchItemBodySection{s.bodySection}}
	msgs, err := s.client.Fetch(imap.UIDSetNum(uid), opts).Collect()
	if err != nil {
		return nil, fmt.Errorf("imap fetch uid %d: %w", uid, err)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("imap fetch uid %d: no message returned", uid)
	}
	body := msgs[0].FindBodySection(s.bodySection)
	if body == nil {
		return nil, fmt.Errorf("imap fetch uid %d: empty body", uid)
	}
	return body, nil
}

func (s *imapClientSession) Close() error {
	_ = s.client.Logout().Wait()
	return s.client.Close()
}
```

> **go-imap API note (verified against pkg.go.dev for the imapclient package):** the calls above match v2.0.0-beta.8 — `imapclient.DialTLS/DialStartTLS(addr, *imapclient.Options)`, `Client.Login(u,p).Wait()`, `Client.Select(name, *imap.SelectOptions{ReadOnly:true}).Wait()`, `Client.UIDSearch(&imap.SearchCriteria{}, nil).Wait()` → `*imap.SearchData` with `.AllUIDs() []imap.UID`, `Client.Fetch(imap.UIDSetNum(uid), *imap.FetchOptions).Collect()` → `[]*imapclient.FetchMessageBuffer` with `.FindBodySection(section) []byte`, `imap.FetchItemBodySection{Specifier: imap.PartSpecifierFull, Peek: true}`, `Client.Logout().Wait()`, `Client.Close()`. If any symbol differs in the installed beta, adjust the **adapter only** (`newIMAPSession`/`imapClientSession`) to the real API — it is behind the `imapSession` interface and the unit tests use a fake, so the source logic and tests are unaffected. The build (Step 4) is the gate.

- [ ] **Step 4: Tidy modules, build, test**

Run:
```bash
go mod tidy
CGO_ENABLED=0 go build ./...
CGO_ENABLED=1 go test ./internal/importer/... -run IMAP -count=1
CGO_ENABLED=1 go test ./internal/importer/... -count=1
```
Expected: `go mod tidy` promotes go-imap to a direct require; build exit 0; IMAP tests pass; full importer package passes.

- [ ] **Step 5: Commit**

```bash
git add internal/importer/source_imap.go internal/importer/source_imap_test.go go.mod go.sum
git commit -m "feat(importer): IMAP Source (read-only, lazy UID fetch)"
```

---

## Task 3: Wire handler + route

**Files:**
- Modify: `internal/server/handlers_imports.go`
- Modify: `internal/server/router.go`

Context: `handleImportS3` already exists in handlers_imports.go and is the exact template — `runJob(importID, src, cleanup)` and `newImportID()`/`writeJSON()` are in this file; imports `json`, `strings`, `fmt`, `importer`, `store`, `net/http` are present.

- [ ] **Step 1: Add the IMAP handler**

Append to `internal/server/handlers_imports.go`:
```go
func (s *Server) handleImportImap(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		Folder   string `json:"folder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "decode body: "+err.Error(), http.StatusBadRequest)
		return
	}
	body.Host = strings.TrimSpace(body.Host)
	body.Username = strings.TrimSpace(body.Username)
	body.Folder = strings.TrimSpace(body.Folder)
	if body.Host == "" || body.Username == "" || body.Password == "" {
		http.Error(w, "host, username and password are required", http.StatusBadRequest)
		return
	}

	folder := body.Folder
	if folder == "" {
		folder = "INBOX"
	}
	importID := newImportID()
	sourceName := fmt.Sprintf("imap://%s@%s/%s", body.Username, body.Host, folder)
	if err := s.Store.CreateImport(r.Context(), store.Import{
		ID:         importID,
		SourceKind: "imap",
		SourceName: sourceName,
		Status:     "queued",
	}); err != nil {
		http.Error(w, "create import: "+err.Error(), http.StatusInternalServerError)
		return
	}

	src := importer.NewIMAPSource(importer.IMAPConfig{
		Host:     body.Host,
		Port:     body.Port,
		Username: body.Username,
		Password: body.Password,
		Folder:   body.Folder,
	})

	go s.runJob(importID, src, func() {})

	writeJSON(w, http.StatusAccepted, map[string]any{
		"import_id": importID,
		"kind":      "imap",
	})
}
```

- [ ] **Step 2: Register the route**

In `internal/server/router.go`, after the line `api.Post("/imports/s3", s.handleImportS3)`, add:
```go
		api.Post("/imports/imap", s.handleImportImap)
```

- [ ] **Step 3: Build, vet, test**

Run:
```bash
CGO_ENABLED=0 go build ./... && go vet ./... && CGO_ENABLED=1 go test ./... -race -count=1
```
Expected: build clean, vet clean, all packages pass.

- [ ] **Step 4: Commit**

```bash
git add internal/server/handlers_imports.go internal/server/router.go
git commit -m "feat(server): POST /api/imports/imap"
```

---

## Task 4: Frontend API client

**Files:** Modify `web/src/lib/api.ts`

- [ ] **Step 1: Add the type and method**

In `web/src/lib/api.ts`, add this interface near the other interfaces (e.g. after `S3ImportConfig`):
```ts
export interface IMAPImportConfig {
  host: string
  port?: number
  username: string
  password: string
  folder?: string
}
```

Add this method to the `api` object, right after `uploadS3(...)`:
```ts
  async uploadImap(cfg: IMAPImportConfig): Promise<{ import_id: string; kind: string }> {
    const res = await fetch(`${BASE}/api/imports/imap`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cfg),
    })
    return jsonOrThrow(res)
  },
```

- [ ] **Step 2: Type-check**

Run: `cd web && npm run type-check`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/api.ts
git commit -m "feat(web): api.uploadImap client"
```

---

## Task 5: Frontend composable

**Files:** Modify `web/src/composables/useImports.ts`

Context: the file imports `{ api, type ImportEvent, type S3ImportConfig }` and `useImports()` returns `{ runs, startImport, startS3Import }`. `startS3Import` is the template (POST, build run with total 0, followProgress).

- [ ] **Step 1: Update the import and add startImapImport**

Change the api import line to also import the type:
```ts
import { api, type ImportEvent, type S3ImportConfig, type IMAPImportConfig } from '@/lib/api'
```

Inside `useImports()`, add this function (next to `startS3Import`) and add `startImapImport` to the returned object:
```ts
  async function startImapImport(cfg: IMAPImportConfig) {
    const { import_id, kind } = await api.uploadImap(cfg)
    const run = reactive<ImportRun>({
      id: import_id,
      kind,
      phase: t('import.queued'),
      current: '',
      total: 0,
      processed: 0,
      duplicates: 0,
      errors: 0,
      done: false,
      log: [],
    })
    runs.value.unshift(run)
    followProgress(run)
  }
```
The return statement becomes:
```ts
  return { runs, startImport, startS3Import, startImapImport }
```

- [ ] **Step 2: Type-check**

Run: `cd web && npm run type-check`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/composables/useImports.ts
git commit -m "feat(web): startImapImport in useImports"
```

---

## Task 6: Import page — third provider

**Files:** Modify `web/src/pages/ImportPage.vue`

Context: the page already has a provider toggle with `'local' | 's3'` and a two-step S3 form. This task adds `'imap'` as a third option and an IMAP form mirroring the S3 pattern. READ the current file first to anchor edits.

- [ ] **Step 1: Extend the Provider type and add IMAP state (script)**

In `<script setup>`, change:
```ts
type Provider = 'local' | 's3'
```
to:
```ts
type Provider = 'local' | 's3' | 'imap'
```

Then, immediately after the S3 block (after the `inputClass` const, or after the `awsRegions` const if present — i.e. at the end of the script block, before `</script>`), add the IMAP state and handlers:
```ts
// --- IMAP provider ---
const imapForm = ref<IMAPImportConfig>({
  host: '',
  port: 993,
  username: '',
  password: '',
  folder: 'INBOX',
})
const imapConfirming = ref(false)

const imapValid = computed(
  () =>
    imapForm.value.host.trim().length > 0 &&
    imapForm.value.username.trim().length > 0 &&
    imapForm.value.password.length > 0,
)

function reviewImap() {
  if (imapValid.value) imapConfirming.value = true
}

function cancelImap() {
  imapConfirming.value = false
}

async function confirmImap() {
  const cfg: IMAPImportConfig = {
    host: imapForm.value.host.trim(),
    username: imapForm.value.username.trim(),
    password: imapForm.value.password,
    port: imapForm.value.port || undefined,
    folder: imapForm.value.folder?.trim() || undefined,
  }
  imapConfirming.value = false
  await startImapImport(cfg)
}
```

Update the imports at the top of the script:
- change `import type { S3ImportConfig } from '@/lib/api'` to `import type { S3ImportConfig, IMAPImportConfig } from '@/lib/api'`
- change `const { runs, startImport, startS3Import } = useImports()` to `const { runs, startImport, startS3Import, startImapImport } = useImports()`

- [ ] **Step 2: Add the IMAP toggle button (template)**

In the provider toggle div (the `inline-flex ... bg-muted` block with the two `<button>`s), add a third button after the S3 one:
```html
      <button :class="providerBtnClass('imap')" @click="provider = 'imap'">
        {{ t('import.provider_imap') }}
      </button>
```

- [ ] **Step 3: Add the IMAP pane (template)**

Immediately AFTER the closing `</template>` of the S3 `<template v-else>` block (i.e. after the S3 confirm `Card`), the current structure is `<template v-else> ...s3... </template>`. Change the S3 wrapper from `v-else` to `v-else-if="provider === 's3'"`, then add a new IMAP block after it:
```html
    <!-- IMAP provider -->
    <template v-else>
      <Card v-if="!imapConfirming" class="p-6 space-y-4">
        <div>
          <h3 class="text-lg font-semibold mb-1">{{ t('import.imap_title') }}</h3>
          <p class="text-sm text-muted-foreground">{{ t('import.imap_hint') }}</p>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.imap_host') }} *</span>
            <input v-model="imapForm.host" :class="inputClass" placeholder="imap.example.com" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.imap_port') }}</span>
            <input v-model.number="imapForm.port" type="number" :class="inputClass" placeholder="993" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.imap_username') }} *</span>
            <input v-model="imapForm.username" :class="inputClass" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.imap_password') }} *</span>
            <input v-model="imapForm.password" type="password" :class="inputClass" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.imap_folder') }}</span>
            <input v-model="imapForm.folder" :class="inputClass" placeholder="INBOX" autocomplete="off" />
          </label>
        </div>

        <div class="flex justify-end">
          <Button :disabled="!imapValid" @click="reviewImap">{{ t('import.s3_review') }}</Button>
        </div>
      </Card>

      <Card v-else class="p-6">
        <div class="flex items-start justify-between gap-4 mb-4">
          <div>
            <h3 class="text-lg font-semibold mb-1">{{ t('import.confirm_title') }}</h3>
            <p class="text-sm text-muted-foreground">{{ t('import.imap_confirm_hint') }}</p>
          </div>
          <div class="flex gap-2 shrink-0">
            <Button variant="outline" @click="cancelImap">{{ t('import.cancel') }}</Button>
            <Button @click="confirmImap">{{ t('import.confirm') }}</Button>
          </div>
        </div>
        <dl class="text-sm space-y-2">
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.imap_host') }}</dt>
            <dd class="font-mono">{{ imapForm.host }}</dd>
          </div>
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.imap_username') }}</dt>
            <dd class="font-mono">{{ imapForm.username }}</dd>
          </div>
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.imap_folder') }}</dt>
            <dd class="font-mono">{{ imapForm.folder || 'INBOX' }}</dd>
          </div>
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.imap_port') }}</dt>
            <dd class="font-mono">{{ imapForm.port || 993 }}</dd>
          </div>
        </dl>
      </Card>
    </template>
```

- [ ] **Step 4: Type-check**

Run: `cd web && npm run type-check`
Expected: no errors. (New `import.imap_*` keys are runtime-resolved; type-check is unaffected. They are added in Task 7.)

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/ImportPage.vue
git commit -m "feat(web): IMAP provider tab + form on Import page"
```

---

## Task 7: i18n keys

**Files:** Modify `web/src/locales/en.json`, `web/src/locales/ko.json`

- [ ] **Step 1: Add keys to the `import` object in en.json**

Merge into the existing `"import": { ... }` object in `web/src/locales/en.json`:
```json
    "provider_imap": "IMAP",
    "imap_title": "Import from IMAP",
    "imap_hint": "Connects read-only; messages are never marked read on the server.",
    "imap_host": "Server",
    "imap_port": "Port",
    "imap_username": "Username",
    "imap_password": "Password",
    "imap_folder": "Folder",
    "imap_confirm_hint": "Imports every message in the selected folder."
```

- [ ] **Step 2: Add the matching keys to the `import` object in ko.json**

Merge into the existing `"import": { ... }` object in `web/src/locales/ko.json`:
```json
    "provider_imap": "IMAP",
    "imap_title": "IMAP에서 가져오기",
    "imap_hint": "읽기 전용으로 연결하며, 서버에서 메일을 읽음으로 표시하지 않습니다.",
    "imap_host": "서버",
    "imap_port": "포트",
    "imap_username": "사용자 이름",
    "imap_password": "비밀번호",
    "imap_folder": "폴더",
    "imap_confirm_hint": "선택한 폴더의 모든 메시지를 가져옵니다."
```

- [ ] **Step 3: Validate JSON parity, type-check, build**

Run from repo root:
```bash
node -e "const en=require('./web/src/locales/en.json'),ko=require('./web/src/locales/ko.json');const a=Object.keys(en.import).sort(),b=Object.keys(ko.import).sort();const miss=a.filter(k=>!b.includes(k)).concat(b.filter(k=>!a.includes(k)));if(miss.length){console.error('KEY MISMATCH:',miss);process.exit(1)}console.log('json ok, import keys in parity:',a.length)"
cd web && npm run type-check && npm run build
```
Expected: parity OK, type-check clean, vite build succeeds.

- [ ] **Step 4: Commit**

```bash
git add web/src/locales/en.json web/src/locales/ko.json
git commit -m "feat(web): i18n keys for IMAP import provider"
```

---

## Final verification

- [ ] **Step 1: Backend**

Run: `CGO_ENABLED=0 go build ./... && go vet ./... && CGO_ENABLED=1 go test ./... -race -count=1`
Expected: clean build, clean vet, all tests pass.

- [ ] **Step 2: Frontend**

Run: `cd web && npm run build`
Expected: succeeds.

- [ ] **Step 3: Maintainer smoke test (manual, not run by the agent)**

`make build && ./local-eml serve`; open Import → IMAP; enter host/username/(app-)password and a folder; confirm; watch SSE progress import the folder's messages. Re-running reports duplicates. Verify on the server that messages are NOT marked read (PEEK).

---

## Self-Review Notes

- **Spec coverage:** lazy UID-list-then-fetch (Task 2 `Scan`/`Item.Open`), read-only `EXAMINE` + `BODY.PEEK[]` (Task 2 adapter), `imapSession` test seam with fake (Task 2 tests), `IMAPConfig`/`NewIMAPSource`/`SourceCloser` (Task 2), `imap://user@host/folder` label & stored source_name with no password (Task 2 + Task 3), `POST /api/imports/imap` 202 + host/user/pass validation (Task 3), frontend client/composable/third-tab/i18n (Tasks 4–7). Out-of-scope items (multi-folder, OAuth, incremental, POP3, profiles) are excluded.
- **Type consistency:** `IMAPConfig{Host,Port,Username,Password,Folder}`, `imapSession{UIDs,Fetch,Close}`, `imapSource`, `NewIMAPSource() SourceCloser`, `IMAPImportConfig`, `api.uploadImap`, `startImapImport`, provider `'imap'` are used consistently across tasks. Test helpers `newTestStore`/`newTestPaths`/`importStub`/`minimalEML` are reused from `source_test.go`, not redefined.
- **Password handling:** passed only into `IMAPConfig`; stored row holds only `imap://user@host/folder`; never logged.
- **go-imap beta risk:** confined to the un-unit-tested adapter behind `imapSession`; build is the gate; reference API documented inline.
