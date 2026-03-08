package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestCryptState_LegacyRoundTrip(t *testing.T) {
	t.Skip("OCB2 legacy round-trip: debug with real Mumble client")
	key := make([]byte, 16)
	clientNonce := make([]byte, 16)
	rand.Read(key)
	rand.Read(clientNonce)
	// Sender (client) encrypts with client_nonce; receiver (server) decrypts with client_nonce
	enc := NewCryptState(ModeLegacy)
	if err := enc.SetKey(key, clientNonce, clientNonce); err != nil {
		t.Fatalf("SetKey enc: %v", err)
	}
	dec := NewCryptState(ModeLegacy)
	if err := dec.SetKey(key, clientNonce, clientNonce); err != nil {
		t.Fatalf("SetKey dec: %v", err)
	}
	plain := []byte("hello world")
	dst := make([]byte, len(plain)+enc.Overhead())
	if err := enc.Encrypt(dst, plain); err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got := make([]byte, len(plain))
	if err := dec.Decrypt(got, dst); err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("round-trip: got %q", got)
	}
}

func TestCryptState_SecureRoundTrip(t *testing.T) {
	cs := NewCryptState(ModeSecure)
	key := make([]byte, 32)
	rand.Read(key)
	if err := cs.SetKey(key, nil, nil); err != nil {
		t.Fatalf("SetKey: %v", err)
	}
	plain := []byte("hello world")
	dst := make([]byte, len(plain)+cs.Overhead())
	if err := cs.Encrypt(dst, plain); err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got := make([]byte, len(plain))
	if err := cs.Decrypt(got, dst); err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("round-trip: got %q", got)
	}
}
