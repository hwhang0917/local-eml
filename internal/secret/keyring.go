package secret

import (
	"errors"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"
)

// Keyring is the slice of an OS secret store the key loader needs. The real
// one is OSKeyring; tests pass an in-memory fake.
type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, secret string) error
	Delete(service, user string) error
}

// ErrKeyNotFound is what a Keyring returns when it has no entry. Any other
// error means the store itself could not be reached.
var ErrKeyNotFound = keyring.ErrNotFound

// KeyringService is the service name the key is filed under: what shows in
// Windows Credential Manager, Keychain Access or Seahorse.
const KeyringService = "local-eml"

// keyringTimeout bounds every call. On Linux a locked or half-installed
// Secret Service can wait forever on an unlock prompt nobody will see (a
// systemd unit, WSL), and D-Bus activation of a missing service takes ~25s.
const keyringTimeout = 5 * time.Second

// OSKeyring is the platform store: Credential Manager on Windows, the login
// Keychain on macOS, Secret Service over D-Bus on Linux. All three are pure
// Go in zalando/go-keyring, so release builds stay CGO_ENABLED=0.
type OSKeyring struct{}

func (OSKeyring) Get(service, user string) (string, error) {
	return withTimeout(func() (string, error) { return keyring.Get(service, user) })
}

func (OSKeyring) Set(service, user, secret string) error {
	_, err := withTimeout(func() (string, error) { return "", keyring.Set(service, user, secret) })
	return err
}

func (OSKeyring) Delete(service, user string) error {
	_, err := withTimeout(func() (string, error) { return "", keyring.Delete(service, user) })
	return err
}

var errKeyringTimeout = errors.New("OS keychain did not answer")

// withTimeout runs fn on its own goroutine so a hung store can be abandoned.
// The goroutine leaks until the store answers; that is at most one per call
// and only on a broken setup.
func withTimeout(fn func() (string, error)) (string, error) {
	type result struct {
		s   string
		err error
	}
	done := make(chan result, 1)
	go func() {
		s, err := fn()
		done <- result{s, err}
	}()
	select {
	case r := <-done:
		return r.s, r.err
	case <-time.After(keyringTimeout):
		return "", fmt.Errorf("%w within %s", errKeyringTimeout, keyringTimeout)
	}
}
