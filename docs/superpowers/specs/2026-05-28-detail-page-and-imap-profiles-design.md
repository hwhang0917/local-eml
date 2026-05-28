# Separate Detail Page + Named IMAP Profiles — Design

Date: 2026-05-28

## Goal

Two independent changes shipped together:

1. **Revert the master-detail split-panel layout.** The email detail returns to
   its own page (`/email/:sha`) reached by same-tab navigation from the library.
   The pop-out / chrome-less window feature is removed entirely. Filter / search
   / sort / pagination continue to round-trip via the URL query string, so
   browser back restores the library exactly as it was — which was the original
   ask before the split-panel pivot.
2. **Persist IMAP connection settings as named profiles.** A new SQLite table
   stores host / port / username / folder under a user-supplied name; passwords
   are never written to disk. The Import page IMAP form gains a profile
   dropdown + Save / Delete buttons so a connection can be reused.

Backend is touched only by Change 2 (new table + endpoints). Change 1 is
frontend-only.

## Background

Today, after the v0.2.0 commits:

- `LibraryPage.vue` lays out as `flex gap-6`: collapsible tags `Sidebar`, then
  a `SplitterGroup` wrapping a list `<section>` and a detail `<aside>` rendering
  `<EmailDetail :sha="selected">` (selection in `?sel=<sha>`,
  `replaceQuery({ sel: … })` toggle).
- `EmailDetail.vue` is a reusable component with `props: { sha: string;
  standalone?: boolean }`. When `standalone`, it owns `document.title` and shows
  a pop-out button that opens `/email/:sha` in a chrome-less new window.
- `ViewerPage.vue` is a thin wrapper rendering `<EmailDetail :sha standalone />`.
- `router.ts` marks the viewer route `meta: { chromeless: true }`.
- `App.vue` branches on `route.meta.chromeless` and renders either a tight
  `<RouterView />` (no header/nav) or the full layout. A single `<Toaster>` is
  hoisted as a sibling after both branches.
- `web/src/lib/app.ts` exports `APP_NAME = 'Local Eml'`.
- The IMAP Import section has host / port / username / password / folder fields
  with manual entry every time; no persistence. The IMAP backend ships through
  `POST /api/imports/imap` with no profile concept.

## Change 1 — Detail returns to its own page

### Component & route changes

- **`LibraryPage.vue`** — drop the right detail pane and the splitter wrapper;
  drop the `?sel` query param, the `selected` computed, and the
  `select(sha)` toggle. The template structure becomes:

  ```
  flex gap-6
  ├─ Sidebar (or collapsed toggle button)   ← unchanged
  └─ section.flex-1.min-w-0                 ← list, full remaining width
  ```

  Remove these imports: `SplitterGroup`, `SplitterPanel`, `SplitterResizeHandle`,
  `EmailDetail`, `Card` (unless still referenced by other parts of the file).
  Row click handler becomes
  `router.push({ name: 'viewer', params: { sha } })`. The row's
  `:class="{ 'bg-accent': e.sha256 === selected }"` highlight goes away (no
  selection state in the list any more).

- **`EmailDetail.vue`** — drop the `standalone?: boolean` prop and drop
  `popOut()` and its pop-out button. The component still imports `APP_NAME`
  from `@/lib/app` (unchanged). The title watcher simplifies to:

  ```ts
  watch(() => email.value?.subject, (subject) => {
    document.title = subject ? `${subject} — ${APP_NAME}` : APP_NAME
  })
  ```

  Add a Back control at the very top of the template that prefers history
  (so the library's filter / search / sort / page query is restored exactly)
  and falls back to a hard navigation when the detail was deep-linked:

  ```ts
  function goBack() {
    if (window.history.length > 1) router.back()
    else router.push('/')
  }
  ```
  ```vue
  <button type="button" @click="goBack"
    class="inline-flex items-center text-sm text-muted-foreground hover:text-foreground mb-4">
    ← {{ t('viewer.back') }}
  </button>
  ```

  Rationale: when the user reached `/email/:sha` by clicking a row, the library
  with its query string sits one step back in history — `router.back()` restores
  it. When the detail was opened fresh (bookmark, address bar), `router.push('/')`
  takes them to a blank-query library rather than out of the app.

- **`ViewerPage.vue`** — stays as the thin wrapper, but loses the `standalone`
  prop: `<EmailDetail :sha="sha" />`.

- **`router.ts`** — remove `chromeless: true` from the viewer route's `meta`.
  The `titleKey: 'nav.viewer'` entry can stay (App.vue won't act on it for this
  route; see below) or be removed. Keep it for consistency; it's harmless.

- **`App.vue`** — collapse the two layout branches into the single full-layout
  branch. The `<Toaster>` becomes a sibling of the single root again. Title
  management: special-case the viewer route in the title watcher to leave the
  title alone (the detail component owns it). Concretely:

  ```ts
  watchEffect(() => {
    if (route.name === 'viewer') return   // EmailDetail owns the title
    const key = (route.meta.titleKey as string) || ''
    document.title = key ? `${t(key)} — ${APP_NAME}` : APP_NAME
  })
  ```

  The chromeless computed and its early-return go away.

### i18n changes (en/ko parity)

- **Restore** `viewer.back` → en: `"Back to library"`, ko: `"라이브러리로 돌아가기"`.
- **Remove** `viewer.pop_out`, `library.select_prompt` (no longer referenced).

### History semantics

- Selecting a row: `router.push` → adds a history entry, so back returns to the
  library with the exact filter / sort / page intact (already in URL).
- Filter / search / tag / page changes inside the library: keep `router.push`
  (unchanged).
- The previous `replaceQuery({ sel })` for selection is removed along with
  the `sel` param.

## Change 2 — Named IMAP profiles (password never persisted)

### Data model

```sql
CREATE TABLE imap_profiles (
  id INTEGER PRIMARY KEY,
  name TEXT UNIQUE NOT NULL,        -- user-supplied; 1..64 chars
  host TEXT NOT NULL,
  port INTEGER,                     -- nullable; 993 used at import time
  username TEXT NOT NULL,
  folder TEXT,                      -- nullable; INBOX used at import time
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
```

Password is deliberately absent. Validation: name 1..64 chars, host
non-empty (after trim), username non-empty (after trim), port (if given) is in
`[1, 65535]`. Rejected requests return 400 with a short JSON body.

Migration: a new numbered migration in `internal/store/sqlite.go`'s migration
table (follow whatever pattern the file already uses — same shape as the
existing migration that introduced `tags` / `email_tags`).

### Store

`internal/store/imap_profiles.go`:

```go
type IMAPProfile struct {
    ID        int64
    Name      string
    Host      string
    Port      *int
    Username  string
    Folder    *string
    CreatedAt int64
    UpdatedAt int64
}

func (s *Store) ListIMAPProfiles(ctx context.Context) ([]IMAPProfile, error)
func (s *Store) UpsertIMAPProfile(ctx context.Context, p IMAPProfile) (IMAPProfile, error)  // by name; INSERT OR REPLACE preserving created_at on update
func (s *Store) DeleteIMAPProfile(ctx context.Context, id int64) error
```

Upsert logic: lookup-by-name; if exists, `UPDATE … SET host=…, port=…,
username=…, folder=…, updated_at=…` keeping the same id and `created_at`;
otherwise `INSERT` with both timestamps set to now. Return the resulting row.

### Server

`internal/server/handlers_imap_profiles.go` exposes:

| Method | Path | Body | Response |
|---|---|---|---|
| `GET` | `/api/imap/profiles` | — | `200 [{id,name,host,port?,username,folder?}, …]` |
| `POST` | `/api/imap/profiles` | `{name,host,port?,username,folder?}` | `200 {id,…}` |
| `DELETE` | `/api/imap/profiles/:id` | — | `204` |

Wire them up in `internal/server/router.go` alongside the existing imports
endpoints. `:id` is parsed as an int64; non-numeric returns 400, missing row on
delete returns 404. JSON response shape uses `omitempty` for nullable `port` /
`folder`. Test the handlers in `internal/server/handlers_imap_profiles_test.go`
covering create, list-includes-it, update-by-same-name, delete, and the four
validation paths (blank name, blank host, blank username, out-of-range port).

### Frontend API client

In `web/src/lib/api.ts`:

```ts
export interface IMAPProfile {
  id: number
  name: string
  host: string
  port?: number
  username: string
  folder?: string
}
// password is NEVER part of this interface

api.listIMAPProfiles(): Promise<IMAPProfile[]>
api.saveIMAPProfile(p: Omit<IMAPProfile, 'id'>): Promise<IMAPProfile>
api.deleteIMAPProfile(id: number): Promise<void>
```

### UI — `ImportPage.vue` IMAP section

Add a profile row above the existing fields:

```
┌────────────────────────────────────────────────────────┐
│  Profile:  [▾ — New —          ]   [ Save ]   [Delete] │
└────────────────────────────────────────────────────────┘
   Host *           Port
   [imap.example.com]   [993]
   Username *       Password *
   [             ]   [        ]
   Folder
   [INBOX]
```

Behavior:

- On mount, call `api.listIMAPProfiles()`; store in a `profiles` ref.
- Local state: `selectedProfileId: number | null` (null = "— New —").
- Selecting a profile fills `imapForm.host/port/username/folder` from its row;
  `imapForm.password` is **always** cleared (and the password field gets a
  visible focus to remind the user to type it).
- Editing any field while a profile is selected is allowed (fields are not
  locked); on Save the form values overwrite that profile's stored values.
- **Save** button:
  - If `selectedProfileId === null` ("— New —"): use the browser `prompt()` for
    a name (with a localized title); cancel aborts.
  - If a profile is selected: re-save under its existing name (no prompt).
  - POST to `/api/imap/profiles`. On success, refresh `profiles` and set
    `selectedProfileId` to the returned id. Toast on failure.
- **Delete** button:
  - Disabled when `selectedProfileId === null`.
  - Native `confirm()` ("Delete profile X?"). DELETE on accept. On success,
    drop from `profiles`, reset to "— New —". Toast on failure.
- Saving never sends the password to `/api/imap/profiles`; the password lives
  only in `imapForm.password` and is sent only on the import POST.

`browser prompt()` / `confirm()` are acceptable here (the app is a local single-
user tool; this is consistent with how the rest of the UI handles confirmations
like attachment overrides). A custom modal can come later if needed.

### i18n (en/ko parity)

- `import.imap_profile` — "Profile" / "프로필"
- `import.imap_profile_new` — "— New —" / "— 새 프로필 —"
- `import.imap_profile_save` — "Save" / "저장"
- `import.imap_profile_save_prompt` — "Profile name:" / "프로필 이름:"
- `import.imap_profile_delete` — "Delete" / "삭제"
- `import.imap_profile_delete_confirm` — "Delete profile \"{name}\"?" / "\"{name}\" 프로필을 삭제할까요?"
- `import.imap_profile_save_error` — "Couldn't save profile" / "프로필을 저장하지 못했어요"
- `import.imap_profile_delete_error` — "Couldn't delete profile" / "프로필을 삭제하지 못했어요"

### Security notes

- Stored fields (host, port, username, folder, profile name) are not secrets.
  Host is a public DNS name; username is typically an email address. These are
  the same kinds of fields users routinely put in mail-client setup screens.
- The password is held only in the in-memory form state, sent over the
  loopback HTTP boundary to the existing `/api/imports/imap` endpoint, and
  forgotten when the import worker finishes. No log lines should include
  passwords (verify the import handler doesn't print the request body).
- The SQLite DB lives under the user's home directory (same place as the
  email blobs); file permissions match the existing DB file. No additional
  at-rest encryption is in scope for v1.

## Testing

Backend: standard Go unit tests for the store layer (table-tested:
upsert-creates, upsert-updates-by-name-preserves-created-at, delete-removes,
list-orders-by-name) and handler tests covering the 200/204/400/404 paths.
`go test ./...` remains green.

Frontend: no test framework exists. Gate is `npm run type-check` (vue-tsc).
Manual checks the maintainer should run:

- Open `/` → click a row → navigated to `/email/:sha`; browser back returns to
  the same filtered / sorted / paginated list. Open with a URL containing
  `?q=foo&sort=subject&order=asc&offset=50` → load shows that view; click a
  row, then Back link → query intact.
- The pop-out button is gone; `/email/:sha` shows the normal app header + nav.
- Open `/import` → IMAP tab → enter a new connection → Save (prompt asks for a
  name) → reload page → dropdown shows the saved name → select it → host /
  port / username / folder are filled, password is empty. Edit folder → Save
  (no prompt) → reload → folder change persisted. Delete → confirm → entry
  gone from dropdown. SQLite file inspected: `SELECT * FROM imap_profiles`
  has no `password` column.

## Out of scope (v1)

- Encrypted-at-rest profile storage (the DB lives under the user's home dir;
  metadata only).
- Optional OS keychain (Keychain / Credential Manager / Secret Service)
  integration for the password.
- Profile rename (delete + save as new name covers this).
- TLS / STARTTLS toggle per profile (uses current default).
- Importing / exporting profiles across machines.
- A custom in-app modal replacing `prompt()` / `confirm()` for the Save and
  Delete flows.
