# Security Modes

> **Status:** Implemented

## Overview

go-mumble-server supports three protocol security tiers that are **negotiated per client** during the Version exchange. The server always accepts all clients and selects the best mutually-supported crypto tier for each connection. There is no server-wide security mode setting.

| Tier | Name | UDP Cipher | TLS | Client Cert | Target |
|------|------|-----------|-----|-------------|--------|
| 0 | `lite` | None (cleartext) | TLS 1.2+ | Optional | ESP32, constrained IoT |
| 1 | `legacy` | OCB2-AES128 | TLS 1.2+ | Optional | Standard Mumble clients |
| 2 | `secure` | AES-256-GCM | TLS 1.3 | Required | Security-aware clients |

**Default:** Tier 1 (legacy) — any client that does not advertise capabilities is treated as a standard Mumble client and uses legacy crypto.

## Negotiation Flow

1. Client connects via TLS (1.2 or 1.3; server accepts both).
2. Server sends `Version` with `CryptoModes = 0x07` (all tiers supported).
3. Client sends `Version` with optional `CryptoModes` bitmask (bit 0 = lite, bit 1 = legacy, bit 2 = secure).
4. Client sends `Authenticate`.
5. Server selects the best tier: secure (if TLS 1.3 + client cert + client advertised secure), else legacy, else lite.
6. Server sends `CryptSetup` with key size indicating mode: 32 bytes = secure, 16 bytes = legacy, 0 bytes = lite.

Clients that omit `CryptoModes` (standard Mumble clients) default to legacy only.

## Legacy Mode (Tier 1)

Legacy mode implements the exact Mumble protocol as defined by the upstream project. Any standard Mumble client can connect without modification.

### TLS

- Minimum version: TLS 1.2
- Client certificates optional
- Self-signed server certificates accepted (TOFU model)

### UDP Voice Encryption

- Algorithm: OCB2-AES128
- Key size: 128 bits (16 bytes)
- Tag size: 24 bits (3 bytes), truncated
- Nonce: 128 bits, last byte incremented per packet
- Overhead: 4 bytes per packet

See [encryption.md](encryption.md) for full details.

### Known Weaknesses

These are documented trade-offs accepted for backward compatibility:

| Weakness | Severity | Detail |
|----------|----------|--------|
| OCB2 broken | High | Universal forgery and plaintext recovery attacks (CRYPTO 2019) |
| 3-byte auth tag | Medium | Only 24 bits of authentication |
| SHA-1 fingerprints | Medium | SHA-1 deprecated for collision resistance |

## Secure Mode (Tier 2)

Secure mode uses modern cryptography. It requires the client to advertise the secure capability and to have negotiated TLS 1.3 with a client certificate.

### TLS

- TLS 1.3 only (negotiated by client)
- Client certificate required

### UDP Voice Encryption

- Algorithm: AES-256-GCM
- Key size: 256 bits (32 bytes)
- Nonce: 96 bits (12 bytes)
- Tag: 128 bits (16 bytes)
- Overhead: 28 bytes per packet

### CryptSetup Signal

32-byte key in `CryptSetup` indicates secure mode.

## Lite Mode (Tier 0)

Lite mode sends UDP voice **unencrypted** (cleartext). The control channel remains TLS-encrypted. Intended for constrained devices (e.g. ESP32) on trusted networks.

### Security Warning

Audio is sent in the clear over UDP. Use only on private, trusted networks (e.g. LAN with ESP32 devices).

### CryptSetup Signal

Empty (zero-length) key in `CryptSetup` indicates lite mode.

## Library Support

```go
package crypto

type Mode int

const (
    ModeLite   Mode = iota
    ModeLegacy
    ModeSecure
)

func NewCryptState(mode Mode) *CryptState
func (cs *CryptState) Mode() Mode
func (cs *CryptState) Encrypt(dst, src []byte) error
func (cs *CryptState) Decrypt(dst, src []byte) error
```

## Version Message Extension

Field 6 (`CryptoModes`) is a varint bitmask:

- Bit 0: lite
- Bit 1: legacy
- Bit 2: secure

Standard clients omit this field; the server treats absence as legacy-only.

## Mixed-Mode Channels

When clients using different crypto modes share the same channel (e.g. a lite ESP32 and legacy desktop clients), the server enforces **TCP tunnel relay** for all audio in that channel:

1. **Per-channel tracking** — The server maintains the active crypto mode set for each channel. When a user joins, leaves, or moves channels, the mode set is recomputed.

2. **UDP ping suppression** — When a channel has mixed modes, the server does not echo UDP pings for clients in that channel. Clients that do not receive a ping echo fall back to TCP tunnel (UDPTunnel) automatically per the Mumble protocol.

3. **Forced TCP relay** — Audio destined for recipients in mixed-mode channels is always sent via TCP tunnel (wrapped in the TLS-encrypted control connection) rather than UDP. This ensures each client's security level is maintained without risk of cross-mode packet misinterpretation.

4. **Re-enabling UDP** — When a channel returns to a homogeneous mode (e.g. the lite client leaves), the server resumes echoing UDP pings and sending audio via UDP.

5. **Linked channels** — The mixed-mode check extends to linked channel groups. If channel A (all legacy) is linked to channel B (one lite client), audio routed across the link also uses TCP tunnel.

The channel's aggregate mode is visible in the REST API (`crypto_mode` field on channel nodes) and the web management UI.

## UDP Sender Identification

The server identifies UDP packet senders using a two-tier strategy:

1. **Address cache** — After a client's first UDP packet is identified, the server caches a mapping from the UDP source address to the client's session ID. Subsequent packets from the same address are decrypted using the client's known CryptState (negotiated over TLS) without trial decryption.

2. **Trial decryption fallback** — For unmapped addresses (first packet from a new client), the server tries decryption in priority order: secure, legacy, lite. This identifies the sender and populates the address cache.

3. **NAT rebinding** — If a cached address lookup's decryption fails (source port changed due to NAT), the cache entry is cleared and the server falls back to trial decryption, then re-caches the new address.

## Reference

- OCB2 vulnerability: [Cryptanalysis of OCB2](https://eprint.iacr.org/2019/311) (CRYPTO 2019)
- AES-GCM: [NIST SP 800-38D](https://csrc.nist.gov/publications/detail/sp/800-38d/final)
- TLS 1.3: [RFC 8446](https://www.rfc-editor.org/rfc/rfc8446)
