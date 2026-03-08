# Security Modes

> **Status:** Implemented

## Overview

go-mumble-server supports two protocol security modes, toggled by a single configuration setting. The mode governs TLS policy, UDP voice encryption, certificate hashing, and password hashing for the **wire protocol**. Local storage encryption is always modern regardless of mode.

| Setting | Legacy Mode | Secure Mode |
|---------|-------------|-------------|
| Config value | `security-mode = "legacy"` | `security-mode = "secure"` |
| Standard client compatibility | Full — all existing Mumble clients | Requires secure-mode-aware clients |
| TLS minimum version | TLS 1.2 | TLS 1.3 |
| UDP voice encryption | OCB2-AES128 (3-byte tag) | AES-256-GCM (16-byte tag, 12-byte nonce) |
| Certificate fingerprint | SHA-1 | SHA-256 |
| Password hashing (wire auth) | PBKDF2-SHA256 | Argon2id |
| Key exchange | `CryptSetup` (16-byte key) | `CryptSetup` (32-byte key, extended nonces) |
| Client cert requirements | Optional | Required |

**Default:** `legacy` — ensures drop-in compatibility with all existing Mumble clients.

## Design Principle

The security mode affects only the **protocol layer** — how data moves between client and server over the network. Local storage (the SQLite database, configuration secrets, API keys) is **always** encrypted with modern algorithms regardless of which protocol mode is active. See [Local Storage Encryption](#local-storage-encryption).

## Legacy Mode

Legacy mode implements the exact Mumble protocol as defined by the upstream project. Any standard Mumble client (desktop, Plumble, Mumla) can connect without modification.

### TLS

- Minimum version: TLS 1.2 (matches original Murmur)
- Cipher suites: Go default selection (prefers TLS 1.3 when client supports it, falls back to TLS 1.2)
- Self-signed certificates accepted (TOFU model)
- Client certificates optional

### UDP Voice Encryption

- Algorithm: OCB2-AES128
- Key size: 128 bits (16 bytes)
- Tag size: 24 bits (3 bytes), truncated
- Nonce: 128 bits, last byte incremented per packet
- Overhead: 4 bytes per packet (1 nonce byte + 3 tag bytes)

This is the original Mumble encryption scheme. See [encryption.md](encryption.md) for full details.

### Known Weaknesses (Accepted in Legacy Mode)

These are documented trade-offs accepted for backward compatibility:

| Weakness | Severity | Detail |
|----------|----------|--------|
| OCB2 broken | High | Universal forgery and plaintext recovery attacks (CRYPTO 2019, Inoue/Iwata/Minematsu/Poettering). OCB2 was removed from ISO standards. |
| 3-byte auth tag | Medium | Only 24 bits of authentication. Forgery possible with ~16M attempts. Mitigated by packets being real-time audio (forged packets produce audible garbage). |
| AES-128 key | Low | 128-bit keys are below current 256-bit recommendations but remain computationally secure. |
| Single-byte nonce | Low | Only 256 packets before resync. Functional but fragile under packet loss. |
| SHA-1 fingerprints | Medium | SHA-1 is deprecated for collision resistance. Certificate fingerprints used for user identity. |
| TLS 1.2 permitted | Low | TLS 1.2 with strong cipher suites is acceptable but lacks TLS 1.3 improvements (0-RTT, simplified handshake, removal of legacy ciphers). |

## Secure Mode

Secure mode replaces every weak cryptographic component with modern alternatives following 2025–2026 best practices. It breaks backward compatibility with standard Mumble clients — only clients implementing the secure extensions can connect.

### TLS

- **Minimum version: TLS 1.3 only** — No TLS 1.2 fallback.
- Cipher suites restricted to TLS 1.3 suites (AES-256-GCM, ChaCha20-Poly1305).
- Client certificates **required** — anonymous connections are rejected.
- Server certificate must be valid (self-signed still permitted, but verified on reconnect via pinning).

```go
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{serverCert},
    ClientAuth:   tls.RequireAnyClientCert,
    MinVersion:   tls.VersionTLS13,
    MaxVersion:   tls.VersionTLS13,
}
```

### UDP Voice Encryption

Legacy OCB2-AES128 is replaced with **AES-256-GCM**:

| Property | Legacy (OCB2-AES128) | Secure (AES-256-GCM) |
|----------|---------------------|----------------------|
| Block cipher | AES-128 | AES-256 |
| Mode | OCB2 | GCM |
| Key size | 128 bits (16 bytes) | 256 bits (32 bytes) |
| Nonce size | 128 bits (last byte used) | 96 bits (12 bytes) |
| Tag size | 24 bits (3 bytes) | 128 bits (16 bytes) |
| Overhead per packet | 4 bytes | 28 bytes (12 nonce + 16 tag) |
| Security status | Broken (CRYPTO 2019) | NIST standard, no known attacks |

#### Why AES-256-GCM

- Native hardware acceleration on modern CPUs (AES-NI + CLMUL).
- Available in Go's standard library (`crypto/aes` + `crypto/cipher`).
- NIST approved, FIPS 140-3 compliant.
- Used by TLS 1.3, IPsec, WireGuard (alongside ChaCha20-Poly1305).
- Full 128-bit authentication tag prevents forgery.
- 96-bit explicit nonce eliminates the single-byte nonce wrapping problem.

#### Why Not DTLS

DTLS 1.3 would be the ideal standard for UDP encryption. However:

- Go's standard library does not support DTLS.
- The primary third-party option (pion/dtls) only supports DTLS 1.2, with 1.3 still in progress.
- DTLS adds handshake complexity and MTU fragmentation handling.
- AES-256-GCM with explicit nonces provides equivalent cipher security with the existing key-exchange-over-TLS model.

DTLS 1.3 support may be added as a future option when Go ecosystem support matures.

#### Secure CryptState

```go
type CryptState struct {
    Key            [32]byte       // AES-256 key
    EncryptCounter uint64         // monotonic counter for nonce generation
    DecryptWindow  [64]uint64     // replay detection bitmap
    DecryptMax     uint64         // highest accepted nonce
    Good, Late, Lost, Resync uint32
}
```

Nonce construction: 12-byte nonce = 4-byte zero prefix + 8-byte big-endian counter. The counter is monotonically increasing and never reuses a value. The full 12-byte nonce is transmitted with each packet (no reconstruction needed).

#### Secure UDP Packet Format

```
┌────────────────────────────────────────────┐
│ Nonce (12 bytes)                           │  Explicit, full nonce
├────────────────────────────────────────────┤
│ Encrypted payload (variable)               │  AES-256-GCM ciphertext
├────────────────────────────────────────────┤
│ Authentication tag (16 bytes)              │  Full GCM tag
└────────────────────────────────────────────┘
```

Total overhead: 28 bytes per packet (12 nonce + 16 tag) vs 4 bytes in legacy mode. For typical voice packets (50–200 bytes), this is acceptable.

### Certificate Fingerprints

- **SHA-256** replaces SHA-1 for certificate fingerprint computation.
- The `hash` field in `UserState` messages carries the SHA-256 hex digest.
- Registered users are bound to their SHA-256 fingerprint.
- Clients connecting in secure mode must present a certificate with a SHA-256 fingerprint.

### Password Hashing

- **Argon2id** replaces PBKDF2 for password verification during authentication.
- Parameters: 64 MiB memory, 3 iterations, parallelism matching available cores.
- 16-byte random salt, 32-byte derived key.
- Stored as a PHC string: `$argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>`.

Note: Password hashing affects the authentication check on the wire protocol (the `Authenticate` message password field). The storage format for password hashes is always modern — see [Local Storage Encryption](#local-storage-encryption).

### Key Exchange

In secure mode, the `CryptSetup` message carries larger fields:

| Field | Legacy | Secure |
|-------|--------|--------|
| `key` | 16 bytes (AES-128) | 32 bytes (AES-256) |
| `client_nonce` | 16 bytes | 12 bytes (GCM nonce) |
| `server_nonce` | 16 bytes | 12 bytes (GCM nonce) |

The `CryptSetup` message uses `bytes` fields, so the larger key is wire-compatible. The security mode determines how these fields are interpreted.

## Local Storage Encryption

Regardless of protocol security mode, all local storage uses modern encryption:

### Database

- SQLite database encrypted at rest using **SQLCipher** (AES-256-CBC with HMAC-SHA512) or application-level encryption with AES-256-GCM.
- Encryption key derived from a server master key.
- Master key stored in a separate key file with restricted filesystem permissions (0600).
- Database contains: registered users, certificates, channel tree, ACLs, groups, bans, configuration, logs.

### Password Storage

Passwords are **always** stored using Argon2id, even when the server runs in legacy mode. Legacy mode only affects the wire protocol authentication check — the stored hash is always modern.

| Aspect | Legacy Wire | Secure Wire | Storage |
|--------|-------------|-------------|---------|
| Hash algorithm | PBKDF2 (for compatibility check) | Argon2id | Argon2id (always) |

When running in legacy mode, the server verifies passwords by computing PBKDF2 against the stored Argon2id hash during the authentication exchange. Internally, the canonical password hash is always Argon2id.

### Configuration Secrets

- API keys, server passwords, and TLS private key passphrases in the configuration file can be encrypted.
- Supports environment variable references (`$ENV{VAR}`) to avoid plaintext secrets in config files.
- REST API tokens stored as Argon2id hashes.

### Sensitive Memory

- Cryptographic keys and passwords are zeroed from memory after use (`crypto/subtle.ConstantTimeCompare`, explicit memory clearing).
- Go's garbage collector complicates guaranteed zeroing, but best-effort clearing is applied to key material.

## Mode Negotiation

### Server Configuration

```toml
[security]
mode = "legacy"  # "legacy" or "secure"
```

The mode is a server-wide setting. A server runs in exactly one mode at a time — there is no per-client negotiation. This keeps the security boundary clean and auditable.

### Client Detection

During the connection handshake:

1. Server completes TLS handshake.
2. Server receives `Version` from client.
3. In secure mode, the server checks:
   - TLS version is 1.3 (guaranteed by `tls.Config`).
   - Client presented a certificate (`tls.RequireAnyClientCert`).
   - Client `Version` message includes a secure-mode capability flag.
4. If any check fails in secure mode, the server sends `Reject` with reason and closes.

### Library Support

The protocol library (`pkg/mumble/crypto`) provides both implementations:

```go
package crypto

type Mode int

const (
    ModeLegacy Mode = iota  // OCB2-AES128
    ModeSecure              // AES-256-GCM
)

func NewCryptState(mode Mode) *CryptState
func (cs *CryptState) Encrypt(dst, plaintext []byte) []byte
func (cs *CryptState) Decrypt(dst, ciphertext []byte) ([]byte, error)
```

Both server and client code select the mode at initialization. The framing, handler table, and message encoding layers are mode-agnostic.

## Migration Path

### Legacy → Secure

1. Update server configuration: `mode = "secure"`.
2. Ensure all connecting clients support secure mode.
3. Restart server. Existing legacy clients will be rejected at TLS handshake (TLS 1.3 required) or at version check.
4. No database migration needed — password hashes are already Argon2id internally.
5. Certificate fingerprints are recomputed as SHA-256 on next client connection.

### Secure → Legacy

1. Update server configuration: `mode = "legacy"`.
2. Restart server. Standard Mumble clients can now connect.
3. Password hashes remain Argon2id in storage; legacy wire authentication uses PBKDF2 check against stored hash.

## Comparison Summary

| Component | Legacy Mode | Secure Mode | Local Storage |
|-----------|-------------|-------------|---------------|
| TLS | 1.2+ | 1.3 only | N/A |
| UDP cipher | OCB2-AES128 | AES-256-GCM | N/A |
| UDP key size | 128-bit | 256-bit | N/A |
| UDP tag size | 3 bytes (24-bit) | 16 bytes (128-bit) | N/A |
| UDP nonce | 1 byte (reconstructed) | 12 bytes (explicit) | N/A |
| Cert fingerprint | SHA-1 | SHA-256 | N/A |
| Client certs | Optional | Required | N/A |
| Password hash (wire) | PBKDF2 | Argon2id | Argon2id (always) |
| Database encryption | N/A | N/A | AES-256 (always) |
| Config secrets | N/A | N/A | Encrypted or env refs (always) |
| Memory clearing | N/A | N/A | Best-effort zeroing (always) |

## Reference

- OCB2 vulnerability: [Cryptanalysis of OCB2](https://eprint.iacr.org/2019/311) (CRYPTO 2019)
- Mumble OCB2 issue: [mumble-voip/mumble#4219](https://github.com/mumble-voip/mumble/issues/4219)
- AES-GCM: [NIST SP 800-38D](https://csrc.nist.gov/publications/detail/sp/800-38d/final)
- Argon2id: [RFC 9106](https://www.rfc-editor.org/rfc/rfc9106)
- TLS 1.3: [RFC 8446](https://www.rfc-editor.org/rfc/rfc8446)
- Go crypto/tls: [pkg.go.dev/crypto/tls](https://pkg.go.dev/crypto/tls)
- Go crypto/cipher (GCM): [pkg.go.dev/crypto/cipher](https://pkg.go.dev/crypto/cipher)
