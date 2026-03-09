# 0002: Negotiated Per-Client Security Modes

## Status: Implemented

## Summary

Removed the server-level security-mode configuration and replaced it with per-client security negotiation using the Version message exchange. The server always accepts all clients and negotiates the best mutually-supported crypto tier per connection. Added a "lite" tier for constrained devices (e.g. ESP32).

## Security Tiers

| Tier | Name | UDP Cipher | TLS | Target |
|------|------|-----------|-----|--------|
| 0 | lite | None (cleartext) | TLS 1.2+ | ESP32, constrained IoT |
| 1 | legacy | OCB2-AES128 | TLS 1.2+ | Standard Mumble clients |
| 2 | secure | AES-256-GCM | TLS 1.3 + client cert | Security-aware clients |

## Changes

- Removed `SecurityMode` from config structs, TOML, env vars, DB model, REST API, frontend
- TLS listener always uses permissive TLS (TLS 1.2+, optional client certs)
- Added `CryptoModes` field (field 6) to Version message for capability negotiation
- Added `ModeLite` to CryptState (no-op encrypt/decrypt, zero overhead)
- `negotiateCryptoMode` selects best tier based on client capabilities and TLS state
- HandleUDP tries decryption in order: secure, legacy, lite (to correctly identify senders)

## Compatibility

Standard Mumble clients omit `CryptoModes` and default to legacy. 100% backward compatible.
