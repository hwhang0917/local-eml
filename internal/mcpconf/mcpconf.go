// Package mcpconf stores whether AI agents may connect over MCP, and the
// token they must present. It is a small JSON file in the owner-only data
// folder, like the session token, rather than a settings row: settings travel
// in backups, and restoring an old backup must never switch agent access on
// or bring back a token the user has since rotated.
//
// The token is separate from the session token on purpose. The session token
// opens the whole API, including wiping the library; the MCP token opens only
// the read-only tools at /mcp, so pasting it into an agent's config file
// hands over no more than that.
package mcpconf

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	Enabled bool   `json:"enabled"`
	Token   string `json:"token,omitempty"`
}

const (
	fileMode   = 0o600
	dirMode    = 0o700
	tokenBytes = 32
	// TokenPrefix marks the token as ours, so a user (or a secret scanner)
	// who finds it in an agent's config file can tell what it opens.
	TokenPrefix = "leml_mcp_"
)

// Load reads the file. A missing file is the zero value: agents are off.
func Load(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(b, &c)
}

// Save writes c atomically, readable only by its owner.
func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, fileMode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// NewToken mints a random token.
func NewToken() (string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return TokenPrefix + hex.EncodeToString(raw), nil
}
