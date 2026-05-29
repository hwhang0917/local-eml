// Package secret encrypts small secrets (e.g. IMAP passwords) at rest using
// AES-256-GCM with a key stored in a separate file outside the SQLite database.
// A dumped or backed-up database alone therefore does not leak credentials —
// the attacker also needs the keyfile under ~/.local-eml/keys/secret.key.
//
// This is defense-in-depth for a single-user, loopback-only tool. It is not a
// substitute for an OS keychain on a hostile multi-user system.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const keyLen = 32 // AES-256

type Encryptor struct {
	aead cipher.AEAD
}

// Open loads the keyfile at path, creating it (with mode 0600 and 32 random
// bytes) if it doesn't exist. The on-disk format is a single hex line so the
// file is human-inspectable and easy to back up.
func Open(path string) (*Encryptor, error) {
	key, err := loadOrCreateKey(path)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &Encryptor{aead: aead}, nil
}

// Encrypt seals plaintext with a fresh random nonce. Output layout:
// nonce || ciphertext || gcm-tag. Returns nil for empty input so the caller
// can treat "no secret" as zero-value without ambiguity.
func (e *Encryptor) Encrypt(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, nil
	}
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	return e.aead.Seal(nonce, nonce, plain, nil), nil
}

// Decrypt reverses Encrypt. nil/empty input returns nil/no error so callers
// can probe optional fields uniformly.
func (e *Encryptor) Decrypt(blob []byte) ([]byte, error) {
	if len(blob) == 0 {
		return nil, nil
	}
	ns := e.aead.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := blob[:ns], blob[ns:]
	return e.aead.Open(nil, nonce, ct, nil)
}

func loadOrCreateKey(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		return decodeKey(b, path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("rand key: %w", err)
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(key)+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("write key: %w", err)
	}
	return key, nil
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
