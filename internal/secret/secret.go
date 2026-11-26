// Package secret encrypts small secrets (e.g. IMAP passwords) at rest using
// AES-256-GCM. The ciphertext lives in SQLite; the 32-byte key lives apart from
// it, so a dumped or backed-up database alone does not leak credentials.
//
// The key is kept in the OS keychain when one answers (Credential Manager,
// macOS Keychain, Secret Service) and otherwise in a 0600 hex file under
// ~/.local-eml/keys/secret.key. Headless Linux, WSL and containers usually
// have no Secret Service, so they keep using the file. An existing file is
// moved into the keychain on the first start where the keychain works.
//
// This is defense-in-depth for a single-user, loopback-only tool. Any program
// running as the same user can still ask the keychain for the key.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const keyLen = 32 // AES-256

// Where the key is kept, as reported to the Settings page.
const (
	StorageKeychain = "keychain" // OS secret store
	StorageFile     = "file"     // keys/secret.key
	// StorageUnavailable: the key was moved to the keychain earlier but the
	// keychain does not answer now (locked, logged out). Encrypt and Decrypt
	// fail until it does; each call tries again.
	StorageUnavailable = "keychain_unavailable"
)

// ErrKeyUnavailable is returned by Encrypt and Decrypt while the keychain
// holding the key cannot be reached.
var ErrKeyUnavailable = errors.New("encryption key is in the OS keychain, which is not available right now")

type Encryptor struct {
	mu      sync.Mutex
	aead    cipher.AEAD
	storage string
	// retry reloads the key from the keychain while storage is unavailable.
	retry func() ([]byte, error)
}

// Open loads the keyfile at path, creating it (with mode 0600 and 32 random
// bytes) if it doesn't exist. It never touches the OS keychain; see
// OpenKeyring. The on-disk format is a single hex line so the file is
// human-inspectable and easy to back up.
func Open(path string) (*Encryptor, error) {
	key, err := loadOrCreateKey(path)
	if err != nil {
		return nil, err
	}
	return newEncryptor(key, StorageFile)
}

// Options picks where OpenKeyring keeps the key.
type Options struct {
	// KeyFile is the fallback file, and the file an older version left
	// behind. Its absolute path also names the keychain entry, so separate
	// data folders (a temporary session, tests) never share a key.
	KeyFile string
	// Keyring is the OS store; nil means file only.
	Keyring Keyring
	// PreferFile moves a key out of the keychain back into KeyFile, for
	// users who opt out (LOCAL_EML_KEYRING=off).
	PreferFile bool
}

// OpenKeyring loads the key from the keychain or the file, moving it between
// them as Options asks. A marker file next to KeyFile records that the key
// was moved to the keychain, so a keychain that stops answering later is not
// mistaken for a fresh install (which would mint a new key and orphan every
// saved password).
func OpenKeyring(o Options) (*Encryptor, error) {
	if o.Keyring == nil {
		return Open(o.KeyFile)
	}
	abs, err := filepath.Abs(o.KeyFile)
	if err != nil {
		return nil, err
	}
	kc := keychainKey{kr: o.Keyring, account: abs}
	marker := markerPath(o.KeyFile)
	fileKey, fileErr := readKey(o.KeyFile)
	if fileErr != nil && !errors.Is(fileErr, os.ErrNotExist) {
		return nil, fileErr
	}
	_, markerErr := os.Stat(marker)
	moved := markerErr == nil

	switch {
	case fileErr == nil && o.PreferFile:
		// Already in the file; make sure no stale copy lingers in the keychain.
		if moved {
			_ = kc.delete()
			_ = os.Remove(marker)
		}
		return newEncryptor(fileKey, StorageFile)

	case fileErr == nil:
		// Key file from an older version or a keychain-less start: move it.
		if err := kc.store(fileKey); err != nil {
			slog.Info("keeping encryption key in file; OS keychain not available", "err", err)
			return newEncryptor(fileKey, StorageFile)
		}
		if err := writeMarker(marker); err != nil {
			return nil, err
		}
		if err := os.Remove(o.KeyFile); err != nil {
			return nil, fmt.Errorf("remove key file after moving it to the keychain: %w", err)
		}
		slog.Info("moved encryption key into the OS keychain", "file", o.KeyFile)
		return newEncryptor(fileKey, StorageKeychain)

	case moved:
		key, err := kc.load()
		switch {
		case err == nil && o.PreferFile:
			if err := writeKey(o.KeyFile, key); err != nil {
				return nil, err
			}
			_ = kc.delete()
			_ = os.Remove(marker)
			slog.Info("moved encryption key from the OS keychain back to file", "file", o.KeyFile)
			return newEncryptor(key, StorageFile)
		case err == nil:
			return newEncryptor(key, StorageKeychain)
		case errors.Is(err, ErrKeyNotFound):
			// Someone deleted the entry. The saved passwords are lost either
			// way; start over so the user can enter them again.
			slog.Warn("encryption key missing from the OS keychain; saved IMAP passwords must be entered again")
			_ = os.Remove(marker)
			return OpenKeyring(o)
		default:
			slog.Warn("encryption key is in the OS keychain, which is not available; will retry", "err", err)
			return &Encryptor{storage: StorageUnavailable, retry: kc.load}, nil
		}

	default:
		// Fresh start (or the marker was lost). Adopt an existing entry
		// before minting a key, so a lost marker can't orphan passwords.
		if !o.PreferFile {
			if key, err := kc.load(); err == nil {
				if err := writeMarker(marker); err != nil {
					return nil, err
				}
				return newEncryptor(key, StorageKeychain)
			}
		}
		key := make([]byte, keyLen)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, fmt.Errorf("rand key: %w", err)
		}
		if !o.PreferFile {
			err := kc.store(key)
			if err == nil {
				if err := writeMarker(marker); err != nil {
					return nil, err
				}
				return newEncryptor(key, StorageKeychain)
			}
			slog.Info("keeping encryption key in file; OS keychain not available", "err", err)
		}
		if err := writeKey(o.KeyFile, key); err != nil {
			return nil, err
		}
		return newEncryptor(key, StorageFile)
	}
}

// Storage reports where the key is kept: StorageKeychain, StorageFile or
// StorageUnavailable.
func (e *Encryptor) Storage() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.storage
}

func newEncryptor(key []byte, storage string) (*Encryptor, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return &Encryptor{aead: aead, storage: storage}, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return aead, nil
}

// cipher returns the AEAD, retrying the keychain if it was unavailable.
func (e *Encryptor) cipher() (cipher.AEAD, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.aead != nil {
		return e.aead, nil
	}
	if e.retry == nil {
		return nil, ErrKeyUnavailable
	}
	key, err := e.retry()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyUnavailable, err)
	}
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	e.aead, e.storage, e.retry = aead, StorageKeychain, nil
	slog.Info("OS keychain is available again; encryption key loaded")
	return aead, nil
}

// Encrypt seals plaintext with a fresh random nonce. Output layout:
// nonce || ciphertext || gcm-tag. Returns nil for empty input so the caller
// can treat "no secret" as zero-value without ambiguity.
func (e *Encryptor) Encrypt(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, nil
	}
	aead, err := e.cipher()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	return aead.Seal(nonce, nonce, plain, nil), nil
}

// Decrypt reverses Encrypt. nil/empty input returns nil/no error so callers
// can probe optional fields uniformly.
func (e *Encryptor) Decrypt(blob []byte) ([]byte, error) {
	if len(blob) == 0 {
		return nil, nil
	}
	aead, err := e.cipher()
	if err != nil {
		return nil, err
	}
	ns := aead.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := blob[:ns], blob[ns:]
	return aead.Open(nil, nonce, ct, nil)
}

func loadOrCreateKey(path string) ([]byte, error) {
	key, err := readKey(path)
	if !errors.Is(err, os.ErrNotExist) {
		return key, err
	}
	key = make([]byte, keyLen)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("rand key: %w", err)
	}
	return key, writeKey(path, key)
}

func readKey(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeKey(b, path)
}

func writeKey(path string, key []byte) error {
	if err := os.WriteFile(path, []byte(hex.EncodeToString(key)+"\n"), 0o600); err != nil {
		return fmt.Errorf("write key: %w", err)
	}
	return nil
}

// markerPath is the file that says "the key is in the keychain". It holds
// no secret, only a note for anyone looking in the folder.
func markerPath(keyFile string) string {
	return keyFile + ".in-keychain"
}

func writeMarker(path string) error {
	note := "The encryption key for saved IMAP passwords is in the OS keychain\n" +
		"under the service name \"" + KeyringService + "\". Deleting this file does not delete the key.\n"
	if err := os.WriteFile(path, []byte(note), 0o600); err != nil {
		return fmt.Errorf("write keychain marker: %w", err)
	}
	return nil
}

// keychainKey is the one keychain entry holding the hex key.
type keychainKey struct {
	kr      Keyring
	account string
}

func (k keychainKey) load() ([]byte, error) {
	s, err := k.kr.Get(KeyringService, k.account)
	if err != nil {
		return nil, err
	}
	return decodeKey([]byte(s), "keychain entry")
}

// store writes the key and reads it back, so a store that accepts writes but
// returns something else (or nothing) never gets to own the only copy.
func (k keychainKey) store(key []byte) error {
	if err := k.kr.Set(KeyringService, k.account, hex.EncodeToString(key)); err != nil {
		return err
	}
	got, err := k.load()
	if err != nil {
		return fmt.Errorf("read back: %w", err)
	}
	if string(got) != string(key) {
		_ = k.delete()
		return errors.New("read back a different key")
	}
	return nil
}

func (k keychainKey) delete() error {
	return k.kr.Delete(KeyringService, k.account)
}

func decodeKey(raw []byte, path string) ([]byte, error) {
	s := strings.TrimSpace(string(raw))
	key, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decode key %s: %w", path, err)
	}
	if len(key) != keyLen {
		return nil, fmt.Errorf("key %s: wrong length %d, want %d", path, len(key), keyLen)
	}
	return key, nil
}
