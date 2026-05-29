package secret

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestEncryptRoundTrip(t *testing.T) {
	dir := t.TempDir()
	enc, err := Open(filepath.Join(dir, "k"))
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("hunter2")
	blob, err := enc.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, plain) {
		t.Fatalf("ciphertext contains plaintext")
	}
	got, err := enc.Decrypt(blob)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("decrypt=%q want %q", got, plain)
	}
}

func TestEncryptEmpty(t *testing.T) {
	dir := t.TempDir()
	enc, err := Open(filepath.Join(dir, "k"))
	if err != nil {
		t.Fatal(err)
	}
	blob, err := enc.Encrypt(nil)
	if err != nil || blob != nil {
		t.Fatalf("encrypt nil = (%v, %v)", blob, err)
	}
	out, err := enc.Decrypt(nil)
	if err != nil || out != nil {
		t.Fatalf("decrypt nil = (%v, %v)", out, err)
	}
}

func TestKeyfilePersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "k")
	a, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := a.Encrypt([]byte("secret"))

	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.Decrypt(blob)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "secret" {
		t.Fatalf("got %q want secret", got)
	}
}
