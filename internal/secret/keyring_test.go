package secret

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fakeKeyring is an in-memory OS keychain. down makes every call fail the way
// a locked or missing Secret Service does.
type fakeKeyring struct {
	m    map[string]string
	down bool
	// lie makes Get return a different value than Set stored.
	lie bool
}

var errDown = errors.New("org.freedesktop.secrets was not provided")

func newFake() *fakeKeyring { return &fakeKeyring{m: map[string]string{}} }

func (f *fakeKeyring) Get(service, user string) (string, error) {
	if f.down {
		return "", errDown
	}
	v, ok := f.m[service+"/"+user]
	if !ok {
		return "", ErrKeyNotFound
	}
	if f.lie {
		return "00" + v[2:], nil
	}
	return v, nil
}

func (f *fakeKeyring) Set(service, user, secret string) error {
	if f.down {
		return errDown
	}
	f.m[service+"/"+user] = secret
	return nil
}

func (f *fakeKeyring) Delete(service, user string) error {
	if f.down {
		return errDown
	}
	k := service + "/" + user
	if _, ok := f.m[k]; !ok {
		return ErrKeyNotFound
	}
	delete(f.m, k)
	return nil
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

func mustOpen(t *testing.T, o Options) *Encryptor {
	t.Helper()
	e, err := OpenKeyring(o)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func roundTrip(t *testing.T, enc, dec *Encryptor) {
	t.Helper()
	blob, err := enc.Encrypt([]byte("hunter2"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := dec.Decrypt(blob)
	if err != nil || string(got) != "hunter2" {
		t.Fatalf("decrypt = %q, %v", got, err)
	}
}

func TestKeyringFreshInstall(t *testing.T) {
	kf := filepath.Join(t.TempDir(), "secret.key")
	kr := newFake()
	e := mustOpen(t, Options{KeyFile: kf, Keyring: kr})
	if e.Storage() != StorageKeychain {
		t.Fatalf("storage = %s", e.Storage())
	}
	if exists(t, kf) {
		t.Fatal("key file written although the keychain works")
	}
	if !exists(t, markerPath(kf)) {
		t.Fatal("marker missing")
	}
	// The next start reads the same key back.
	roundTrip(t, e, mustOpen(t, Options{KeyFile: kf, Keyring: kr}))
}

func TestKeyringMigratesExistingFile(t *testing.T) {
	kf := filepath.Join(t.TempDir(), "secret.key")
	old, err := Open(kf) // what v0.18 and earlier left behind
	if err != nil {
		t.Fatal(err)
	}
	kr := newFake()
	e := mustOpen(t, Options{KeyFile: kf, Keyring: kr})
	if e.Storage() != StorageKeychain || exists(t, kf) {
		t.Fatalf("storage = %s, file still there = %v", e.Storage(), exists(t, kf))
	}
	// Passwords saved with the file key still decrypt.
	roundTrip(t, old, e)
	roundTrip(t, old, mustOpen(t, Options{KeyFile: kf, Keyring: kr}))
}

func TestKeyringDownUsesFile(t *testing.T) {
	// WSL, headless Linux, a systemd unit before login: no Secret Service.
	kf := filepath.Join(t.TempDir(), "secret.key")
	kr := newFake()
	kr.down = true
	e := mustOpen(t, Options{KeyFile: kf, Keyring: kr})
	if e.Storage() != StorageFile || !exists(t, kf) || exists(t, markerPath(kf)) {
		t.Fatalf("storage = %s", e.Storage())
	}
	roundTrip(t, e, mustOpen(t, Options{KeyFile: kf, Keyring: kr}))

	// Once the keychain shows up, the file moves in and the key is the same.
	kr.down = false
	moved := mustOpen(t, Options{KeyFile: kf, Keyring: kr})
	if moved.Storage() != StorageKeychain || exists(t, kf) {
		t.Fatalf("not moved: %s", moved.Storage())
	}
	roundTrip(t, e, moved)
}

func TestKeyringReadBackMismatchKeepsFile(t *testing.T) {
	kf := filepath.Join(t.TempDir(), "secret.key")
	old, err := Open(kf)
	if err != nil {
		t.Fatal(err)
	}
	kr := newFake()
	kr.lie = true
	e := mustOpen(t, Options{KeyFile: kf, Keyring: kr})
	if e.Storage() != StorageFile || !exists(t, kf) {
		t.Fatalf("a keychain that returns the wrong key must not get the only copy: %s", e.Storage())
	}
	if len(kr.m) != 0 {
		t.Fatal("bad entry left in the keychain")
	}
	roundTrip(t, old, e)
}

func TestKeyringUnavailableAfterMoveRetries(t *testing.T) {
	kf := filepath.Join(t.TempDir(), "secret.key")
	kr := newFake()
	first := mustOpen(t, Options{KeyFile: kf, Keyring: kr})
	blob, err := first.Encrypt([]byte("hunter2"))
	if err != nil {
		t.Fatal(err)
	}

	kr.down = true // locked keychain at the next start
	e := mustOpen(t, Options{KeyFile: kf, Keyring: kr})
	if e.Storage() != StorageUnavailable {
		t.Fatalf("storage = %s", e.Storage())
	}
	if exists(t, kf) {
		t.Fatal("minted a new key file; saved passwords would be orphaned")
	}
	if _, err := e.Decrypt(blob); !errors.Is(err, ErrKeyUnavailable) {
		t.Fatalf("decrypt while down: %v", err)
	}

	kr.down = false // unlocked
	got, err := e.Decrypt(blob)
	if err != nil || !bytes.Equal(got, []byte("hunter2")) {
		t.Fatalf("decrypt after unlock = %q, %v", got, err)
	}
	if e.Storage() != StorageKeychain {
		t.Fatalf("storage after unlock = %s", e.Storage())
	}
}

func TestKeyringEntryDeletedStartsOver(t *testing.T) {
	kf := filepath.Join(t.TempDir(), "secret.key")
	kr := newFake()
	mustOpen(t, Options{KeyFile: kf, Keyring: kr})
	kr.m = map[string]string{}
	e := mustOpen(t, Options{KeyFile: kf, Keyring: kr})
	if e.Storage() != StorageKeychain || len(kr.m) != 1 {
		t.Fatalf("storage = %s, entries = %d", e.Storage(), len(kr.m))
	}
}

func TestKeyringLostMarkerAdoptsEntry(t *testing.T) {
	kf := filepath.Join(t.TempDir(), "secret.key")
	kr := newFake()
	first := mustOpen(t, Options{KeyFile: kf, Keyring: kr})
	if err := os.Remove(markerPath(kf)); err != nil {
		t.Fatal(err)
	}
	roundTrip(t, first, mustOpen(t, Options{KeyFile: kf, Keyring: kr}))
	if !exists(t, markerPath(kf)) {
		t.Fatal("marker not restored")
	}
}

func TestKeyringPreferFileMovesBack(t *testing.T) {
	kf := filepath.Join(t.TempDir(), "secret.key")
	kr := newFake()
	first := mustOpen(t, Options{KeyFile: kf, Keyring: kr})
	e := mustOpen(t, Options{KeyFile: kf, Keyring: kr, PreferFile: true})
	if e.Storage() != StorageFile || !exists(t, kf) || exists(t, markerPath(kf)) || len(kr.m) != 0 {
		t.Fatalf("not moved back: storage=%s entries=%d", e.Storage(), len(kr.m))
	}
	roundTrip(t, first, e)
	// And it stays in the file on the next opted-out start.
	again := mustOpen(t, Options{KeyFile: kf, Keyring: kr, PreferFile: true})
	if again.Storage() != StorageFile || len(kr.m) != 0 {
		t.Fatalf("storage = %s", again.Storage())
	}
	roundTrip(t, first, again)
}

func TestKeyringSeparateFoldersSeparateKeys(t *testing.T) {
	kr := newFake()
	a := mustOpen(t, Options{KeyFile: filepath.Join(t.TempDir(), "secret.key"), Keyring: kr})
	b := mustOpen(t, Options{KeyFile: filepath.Join(t.TempDir(), "secret.key"), Keyring: kr})
	blob, _ := a.Encrypt([]byte("x"))
	if _, err := b.Decrypt(blob); err == nil {
		t.Fatal("two data folders share one keychain key")
	}
	if len(kr.m) != 2 {
		t.Fatalf("entries = %d", len(kr.m))
	}
}
