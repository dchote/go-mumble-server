package ocb2

import (
	"bytes"
	"crypto/aes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	key := make([]byte, 16)
	for i := range key {
		key[i] = byte(i)
	}
	block, _ := aes.NewCipher(key)
	nonce := make([]byte, 16)
	for i := range nonce {
		nonce[i] = byte(i + 1)
	}
	plain := []byte("hello world")
	ciphertext := make([]byte, len(plain))
	tag := make([]byte, MumbleTag)
	Encrypt(block, ciphertext, plain, nonce, tag, MumbleTag)
	got := make([]byte, len(plain))
	if !Decrypt(block, got, ciphertext, nonce, tag) {
		t.Fatal("Decrypt failed (tag mismatch)")
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("got %q, want %q", got, plain)
	}
}
