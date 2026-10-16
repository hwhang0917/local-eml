package server

import (
	"context"
	"testing"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/secret"
	"github.com/hwhang0917/local-eml/internal/store"
)

// A saved password must never be sent anywhere but the server it was saved
// for; a request that overrides the host has to supply its own.
func TestStoredIMAPPasswordOnlyForProfileServer(t *testing.T) {
	s, p := newDriftServer(t)
	enc, err := secret.Open(p.KeyFile())
	if err != nil {
		t.Fatal(err)
	}
	s.Secret = enc
	ctx := context.Background()

	port := 993
	prof, err := s.Store.UpsertIMAPProfile(ctx, store.IMAPProfile{
		Name: "Work", Host: "imap.example.com", Port: &port, Username: "me", SyncEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := enc.Encrypt([]byte("hunter2"))
	if err := s.Store.SetIMAPProfilePassword(ctx, prof.ID, blob); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := s.resolveIMAPConfig(ctx, prof.ID, importer.IMAPConfig{})
	if err != nil || cfg.Password != "hunter2" {
		t.Fatalf("profile alone must use the stored password: cfg=%+v err=%v", cfg, err)
	}

	for name, override := range map[string]importer.IMAPConfig{
		"host": {Host: "imap.attacker.example"},
		"port": {Port: 143},
		"user": {Username: "someone-else"},
	} {
		cfg, _, err := s.resolveIMAPConfig(ctx, prof.ID, override)
		if err == nil || cfg.Password != "" {
			t.Errorf("%s override must not receive the stored password: cfg=%+v err=%v", name, cfg, err)
		}
	}
}
