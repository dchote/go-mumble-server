# 0003: Channel Security Mode Enforcement

## Status: Implemented

## Summary

Added per-channel crypto mode tracking and mixed-mode enforcement. When clients using different crypto tiers (legacy, secure, lite) share a channel, the server forces all audio through TCP tunnel relay to maintain correct encryption for each client. Refactored UDP sender identification to use address-based session lookup instead of trial decryption on every packet.

## Problem

The server negotiates crypto mode per client over TLS (Version exchange + CryptSetup), but the UDP voice handler (`HandleUDP`) ignored this knowledge and identified senders by trial-decrypting every incoming packet against every connected client's CryptState. This was fragile when clients with different modes shared a channel:

- Lite mode `Decrypt()` always succeeds (it is a no-op copy), so it had to be tried last.
- Legacy OCB2 has only a 3-byte authentication tag (24 bits), creating a non-zero probability of false positive decryption on cleartext lite packets.
- Failed OCB2 decrypt calls wrote garbage into the shared plaintext buffer before the tag check rejected them.

## Changes

### Address-Based UDP Session Lookup

- Added `sessionByAddr sync.Map` as a reverse lookup (UDP source address to session ID) alongside the existing `addrBySession`.
- `HandleUDP` primary path: look up session by source address, decrypt with the session's known CryptState. No iteration needed.
- Fallback path: trial decryption for unmapped addresses (first UDP packet from a new client). Populates both maps on success.
- NAT rebinding: if a cached lookup's decrypt fails, the cache entry is cleared and trial decryption is retried.
- Both maps are cleaned up on disconnect.

### Per-Channel Crypto Mode Tracking

- Added `channelCrypto map[uint32]string` on `Server` (channelID to mode string: "legacy", "lite", "secure", "mixed", or empty).
- `UpdateChannelCrypto(channelID)` scans all sessions in the channel and recomputes the aggregate mode.
- Triggered on: user authentication (join), channel move (old + new channel), disconnect (callers invoke `UpdateChannelCrypto` after removing the user), and channel deletion (parent channel updated).

### Mixed-Mode TCP Tunnel Enforcement

- `SendAudio`: checks `channelHasMixedCrypto` for the recipient's channel. When mixed, always uses TCP fallback (`UDPTunnel`).
- `HandleUDP` ping echo: suppresses ping replies for clients in mixed-mode channels, causing them to fall back to TCP tunnel naturally.
- When a channel becomes homogeneous again, UDP is re-enabled via normal ping/echo flow.
- `channelHasMixedCrypto` extends the check to linked channel groups.

### REST API

- Added `crypto_mode` field to `ChannelNode` response (values: "legacy", "lite", "secure", "mixed", or omitted when empty).
- Added `ChannelCryptoLister` interface and `MumbleChannelCryptoAdapter` for wiring.
- Updated OpenAPI spec with `ChannelNode` schema.

### Frontend

- Added a color-coded `v-chip` for channel crypto mode in `ChannelTreeNode.vue` (visible to all users, not gated behind admin).
- Colors: secondary (legacy), info (lite), success (secure), warning (mixed).

### Documentation

- `docs/protocol/security-modes.md`: added Mixed-Mode Channels and UDP Sender Identification sections.
- `docs/product-overview.md`: added mixed-mode enforcement paragraph.
- `docs/technical-overview.md`: updated Audio Router and Crypto sections.
- `README.md`: corrected security mode descriptions, removed inaccurate "encrypted storage" claim.

## Files Modified

- `internal/mumble/handlers.go` — Core changes: address cache, channel crypto tracking, mixed-mode enforcement
- `internal/handler/server.go` — `ChannelNode.CryptoMode` field, `ChannelCryptoLister` interface
- `internal/rest/router.go` — Updated `RouterWithMumble` signature
- `internal/rest/channel_crypto_adapter.go` — New adapter file
- `internal/rest/router_security_test.go` — Updated test call
- `internal/server/server.go` — Wired adapter into REST router
- `api/openapi.yaml` — `ChannelNode` schema with `crypto_mode`
- `frontend/src/components/ChannelTreeNode.vue` — Crypto mode chip

## Compatibility

- Standard Mumble clients: no change. They use legacy mode and are unaffected when all clients in a channel use legacy.
- ESP32 / lite clients: transparent. In mixed-mode channels, they fall back to TCP tunnel (UDPTunnel) which already works. In lite-only channels, UDP works normally.
- REST API: additive change (new optional field). Existing consumers are unaffected.
