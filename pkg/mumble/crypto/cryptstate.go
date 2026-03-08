package crypto

// CryptState handles AEAD encryption/decryption of UDP voice packets.
// Supports legacy mode (OCB2-AES128) and secure mode (AES-256-GCM).
// TODO: implement; see docs/protocol/encryption.md and docs/protocol/security-modes.md
type CryptState struct {
	// key, nonce, and algorithm depend on security mode
}
