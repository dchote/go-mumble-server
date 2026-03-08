# Encryption

> **Status:** Reference — derived from `research/mumble/src/crypto/` with secure mode extensions

## Overview

go-mumble-server uses layered encryption that varies by [security mode](security-modes.md):

1. **TLS** for the TCP control channel — TLS 1.2+ in legacy mode, TLS 1.3 only in secure mode.
2. **AEAD cipher** for UDP voice packets — OCB2-AES128 in legacy mode, AES-256-GCM in secure mode.
3. **Local storage encryption** — AES-256 encrypted database regardless of mode.

The security mode is a server-wide toggle. See [security-modes.md](security-modes.md) for the full comparison and rationale.

## TLS (Control Channel)

### Setup

1. Client connects to the server's TCP port (default 64738).
2. Server presents its TLS certificate.
3. Client optionally (legacy) or mandatorily (secure) presents a client certificate.
4. TLS handshake completes; all subsequent TCP traffic is encrypted.

### Mode Differences

| Aspect | Legacy | Secure |
|--------|--------|--------|
| Minimum TLS | 1.2 | 1.3 |
| Maximum TLS | (any) | 1.3 |
| Client certs | Optional (`RequestClientCert`) | Required (`RequireAnyClientCert`) |
| Cipher suites | Go defaults | TLS 1.3 only (AES-256-GCM, ChaCha20-Poly1305) |

### Go Implementation

```go
// Legacy mode
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{serverCert},
    ClientAuth:   tls.RequestClientCert,
    MinVersion:   tls.VersionTLS12,
}

// Secure mode
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{serverCert},
    ClientAuth:   tls.RequireAnyClientCert,
    MinVersion:   tls.VersionTLS13,
    MaxVersion:   tls.VersionTLS13,
}
```

### Server Certificate

The server uses TLS certificates in the following precedence order:

1. **Configuration files** — If `[tls]` cert and key paths are set, those PEM files are used.
2. **Database persistence** — If no config paths are set, the certificate and key are loaded from the SQLite database (stored per virtual server in `virtual_servers.cert_pem` and `key_pem`).
3. **Generate and persist** — If no cert exists in the database, a self-signed certificate is generated and saved for the virtual server so it persists across restarts.

This ensures the certificate does not change between server restarts, preserving client trust (trust-on-first-use / TOFU model). Clients that pinned the certificate on first connection will continue to connect after a restart.

Configuration (in `mumble-server.toml`):

| Setting | TOML key | Description |
|---------|----------|-------------|
| Certificate | `[tls] cert` | Path to PEM certificate file (overrides DB) |
| Private key | `[tls] key` | Path to PEM private key file (overrides DB) |
| CA | `[tls] ca` | Path to CA certificate for client verification |

### Client Certificates

Client certificates serve as persistent identity:

- The certificate fingerprint uniquely identifies a user.
- **Legacy mode:** SHA-1 of DER-encoded certificate (matches original Mumble).
- **Secure mode:** SHA-256 of DER-encoded certificate.
- Registered users are bound to their certificate — they can reconnect without a password.
- In secure mode, `RequireAnyClientCert` means all users must present a certificate.

## UDP Voice Encryption

### Legacy Mode: OCB2-AES128

UDP voice packets are encrypted with OCB2-AES128 (Offset Codebook Mode, version 2), matching the original Mumble protocol.

Properties:
- **Block cipher:** AES-128
- **Mode:** OCB2
- **Key size:** 128 bits (16 bytes)
- **Nonce size:** 128 bits (16 bytes)
- **Tag size:** 24 bits (3 bytes) — truncated from the full 128-bit tag

**Known vulnerability:** OCB2 was broken at CRYPTO 2019 (Inoue/Iwata/Minematsu/Poettering). Universal forgery and plaintext recovery attacks exist. Retained in legacy mode solely for backward compatibility. See [security-modes.md](security-modes.md) for details.

#### Legacy Key Exchange

The server sends a `CryptSetup` (message type 15) over the TLS-encrypted TCP channel:

| Field | Size | Description |
|-------|------|-------------|
| `key` | 16 bytes | Shared AES-128 key |
| `client_nonce` | 16 bytes | Initial nonce for client→server direction |
| `server_nonce` | 16 bytes | Initial nonce for server→client direction |

Each client has a unique key generated randomly by the server.

#### Legacy Nonce Management

- The nonce is 128 bits (16 bytes).
- The **last byte** of the nonce is incremented for each packet sent.
- The first 15 bytes remain constant (established during `CryptSetup`).
- 256 nonce values before a resynchronization is needed.

#### Legacy Packet Format

```
┌────────────────────────────────────────────┐
│ Nonce byte (1 byte)                        │  Last byte of sender's nonce
├────────────────────────────────────────────┤
│ Authentication tag (3 bytes)               │  Truncated OCB tag
├────────────────────────────────────────────┤
│ Encrypted payload (variable)               │  OCB2-AES128 ciphertext
└────────────────────────────────────────────┘
```

Total overhead: 4 bytes per packet.

### Secure Mode: AES-256-GCM

Secure mode replaces OCB2-AES128 with AES-256-GCM, a NIST-standard AEAD cipher with no known vulnerabilities and hardware acceleration on modern CPUs.

Properties:
- **Block cipher:** AES-256
- **Mode:** GCM (Galois/Counter Mode)
- **Key size:** 256 bits (32 bytes)
- **Nonce size:** 96 bits (12 bytes)
- **Tag size:** 128 bits (16 bytes) — full, not truncated

#### Secure Key Exchange

The `CryptSetup` message carries larger fields in secure mode:

| Field | Size | Description |
|-------|------|-------------|
| `key` | 32 bytes | Shared AES-256 key |
| `client_nonce` | 12 bytes | Initial nonce prefix for client→server |
| `server_nonce` | 12 bytes | Initial nonce prefix for server→client |

The `CryptSetup` message uses `bytes` fields, so larger keys are wire-compatible.

#### Secure Nonce Management

- 12-byte (96-bit) explicit nonce transmitted with every packet.
- Constructed as: 4-byte zero prefix + 8-byte big-endian monotonic counter.
- Counter never wraps — connection is rekeyed before exhaustion (2^64 packets).
- No reconstruction needed; no resynchronization protocol.

#### Secure Packet Format

```
┌────────────────────────────────────────────┐
│ Nonce (12 bytes)                           │  Full explicit nonce
├────────────────────────────────────────────┤
│ Encrypted payload (variable)               │  AES-256-GCM ciphertext
├────────────────────────────────────────────┤
│ Authentication tag (16 bytes)              │  Full GCM tag
└────────────────────────────────────────────┘
```

Total overhead: 28 bytes per packet. For typical voice packets (50–200 bytes payload), this is a ~15–55% size increase — acceptable for the security improvement.

## Replay Protection

Both modes maintain a sliding window to detect replayed, reordered, or late packets.

**Legacy mode:** 256-byte history buffer indexed by nonce byte.

**Secure mode:** 64-entry bitmap window tracking the highest seen nonce counter. Packets with counters below `(max - 64)` are discarded. Packets within the window are checked against the bitmap.

Statistics tracked per connection (both modes):

| Counter | Description |
|---------|-------------|
| `good` | Packets received in order |
| `late` | Packets received out of order (within window) |
| `lost` | Packets never received |
| `resync` | Nonce resynchronization events (legacy only) |

## CryptState

The `CryptState` structure in `pkg/mumble/crypto` supports both modes:

```go
type CryptState struct {
    Mode           Mode
    // Legacy fields (OCB2-AES128)
    LegacyKey      [16]byte
    LegacyEncNonce [16]byte
    LegacyDecNonce [16]byte
    LegacyHistory  [256]byte
    // Secure fields (AES-256-GCM)
    SecureKey      [32]byte
    EncryptCounter uint64
    DecryptWindow  [64]uint64
    DecryptMax     uint64
    // Shared counters
    Good, Late, Lost, Resync uint32
}
```

## Nonce Resynchronization (Legacy Only)

If the receiver falls too far behind (nonce byte wraps around), a resynchronization is triggered:

1. Client sends a `CryptSetup` with just the `client_nonce` field.
2. Server responds with its current `server_nonce`.
3. Both sides resynchronize their nonce state.

In secure mode, explicit 12-byte nonces eliminate the need for resynchronization.

## Password Hashing

### Wire Protocol

- **Legacy mode:** PBKDF2-SHA256 for authentication checks (compatible with original Mumble).
- **Secure mode:** Argon2id for authentication checks.

### Storage (Always Modern)

Passwords are **always** stored as Argon2id hashes regardless of security mode:

- **Algorithm:** Argon2id (RFC 9106)
- **Memory:** 64 MiB
- **Iterations:** 3
- **Parallelism:** 4 (configurable)
- **Salt:** 16 bytes, cryptographically random
- **Key length:** 32 bytes
- **Format:** PHC string `$argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>`

In legacy mode, the server performs the PBKDF2 check using the stored Argon2id hash as the canonical credential store.

## Local Storage Encryption

Regardless of protocol security mode:

- **Database** encrypted at rest using AES-256 (SQLCipher or application-level encryption).
- **Master key** stored in a separate file with restricted permissions (0600).
- **Configuration secrets** support encrypted values or environment variable references.
- **Key material** zeroed from memory after use (best-effort given Go's GC).

See [security-modes.md](security-modes.md) for the full local storage encryption design.

## Reference

- Legacy CryptState: `research/mumble/src/crypto/CryptStateOCB2.h` and `.cpp`
- Legacy key exchange: `CryptSetup` message (see control-messages.md; we use native Go, not protobuf)
- Legacy password hashing: `research/mumble/src/murmur/PBKDF2.cpp`
- OCB2 vulnerability: [eprint.iacr.org/2019/311](https://eprint.iacr.org/2019/311)
- AES-GCM specification: [NIST SP 800-38D](https://csrc.nist.gov/publications/detail/sp/800-38d/final)
- Argon2id specification: [RFC 9106](https://www.rfc-editor.org/rfc/rfc9106)
