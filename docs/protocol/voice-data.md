# Voice Data (UDP / UDPTunnel)

> **Status:** Reference — derived from `research/mumble/src/MumbleUDP.proto` and `research/gumble/gumble/handlers.go`

## Overview

Voice data travels over UDP (encrypted with an AEAD cipher) or is tunneled over the TCP control channel via `UDPTunnel` (message type 1). Two packet formats exist: the legacy binary format and the modern wire format (introduced in Mumble 1.5.0, similar encoding style). The encryption algorithm depends on the [security mode](security-modes.md) — OCB2-AES128 in legacy mode, AES-256-GCM in secure mode.

## Transport

### UDP

- Same port as TCP (default 64738).
- Encrypted with AEAD cipher — OCB2-AES128 (legacy mode) or AES-256-GCM (secure mode). Each client has a unique key and nonce pair.
- Maximum packet size: 1024 bytes.
- Preferred transport for low latency.
- Encryption overhead: 4 bytes (legacy) or 28 bytes (secure). See [encryption.md](encryption.md).

### TCP Tunnel (UDPTunnel)

- Used when UDP is unavailable (NAT, firewall).
- Message type 1 in the TCP framing — the payload is the raw (decrypted) audio packet.
- Higher latency due to TCP head-of-line blocking.
- Clients auto-detect UDP availability and fall back to TCP.

## Legacy Binary Format

Used in all Mumble versions. Still the format inside `UDPTunnel` for legacy clients.

### Packet Structure

```
┌─────────────────────────────────┐
│ Header Byte                     │
│ ┌───────────┬─────────────────┐ │
│ │ Codec (3) │ Target (5 bits) │ │
│ └───────────┴─────────────────┘ │
├─────────────────────────────────┤
│ Session (varint)                │  ← server→client only
├─────────────────────────────────┤
│ Sequence Number (varint)        │
├─────────────────────────────────┤
│ Payload Length (varint)         │  ← bit 13 = terminator flag
├─────────────────────────────────┤
│ Audio Frame Data                │
├─────────────────────────────────┤
│ Positional Audio (optional)     │
│ 3 × float32 (X, Y, Z)         │
│ = 12 bytes, little-endian      │
└─────────────────────────────────┘
```

### Header Byte

| Bits | Field | Description |
|------|-------|-------------|
| 7–5 | Codec | Audio codec identifier |
| 4–0 | Target | Voice target (see below) |

### Codec IDs

| ID | Codec | Notes |
|----|-------|-------|
| 0 | CELT Alpha | Legacy, version-specific |
| 2 | Speex | Deprecated |
| 3 | CELT Beta | Legacy, version-specific |
| 4 | Opus | Preferred, required for modern clients |

### Voice Targets

| Target | Meaning |
|--------|---------|
| 0 | Normal — current channel + linked channels |
| 1–30 | Whisper — uses `VoiceTarget` configuration for this ID |
| 31 | Server loopback — echo back to sender |

### Varint Encoding

Mumble uses a custom varint encoding:

| First byte pattern | Value range | Bytes used |
|---------------------|-------------|------------|
| `0xxxxxxx` | 0 – 127 | 1 |
| `10xxxxxx` + 1 byte | 0 – 16,383 | 2 |
| `110xxxxx` + 2 bytes | 0 – 2,097,151 | 3 |
| `1110xxxx` + 3 bytes | 0 – 268,435,455 | 4 |
| `11110000` + 4 bytes | 32-bit value | 5 |
| `11110100` + 8 bytes | 64-bit value | 9 |
| `11111100` | Negative recursive | variable |
| `11111000` | Negative 2's complement recursive | variable |

Reference: `research/gumble/gumble/varint/read.go` and `write.go`.

### Session Field

- **Client→server:** Session is **not** included. The server identifies the sender by the source UDP address and decryption key.
- **Server→client:** Session **is** included so recipients know who is speaking.

### Terminator Flag

Bit 13 of the payload length varint indicates this is the last audio frame in a speech sequence (the user stopped talking). Clients use this to fade out audio smoothly.

## Modern UDP Format (Protocol 1.5+)

Modern clients may use wire-encoded UDP packets. We implement this with native Go structs; the upstream spec is in `MumbleUDP.proto` (reference only).

### Audio Message

```protobuf
message Audio {
    uint32 target = 1;
    uint32 context = 2;
    uint32 sender_session = 3;
    uint64 frame_number = 4;
    bytes  opus_data = 5;
    repeated float positional_data = 6;
    float  volume_adjustment = 7;
    bool   is_terminator = 8;
}
```

| Field | Description |
|-------|-------------|
| `target` | Voice target (0 = normal, 1–30 = whisper, 31 = loopback) |
| `context` | 0 = normal, 1 = shout to linked channels |
| `sender_session` | Set by server when forwarding |
| `frame_number` | Incrementing frame counter |
| `opus_data` | Opus-encoded audio frame |
| `positional_data` | X, Y, Z coordinates for positional audio |
| `volume_adjustment` | Per-listener volume multiplier |
| `is_terminator` | End of speech sequence |

### Ping Message (UDP)

```protobuf
message Ping {
    uint64 timestamp = 1;
    bool   request_extended_information = 2;
    uint64 server_version_v2 = 3;
    uint32 user_count = 4;
    uint32 max_user_count = 5;
    uint32 max_bandwidth_per_user = 6;
}
```

UDP pings are used for latency measurement and to maintain NAT mappings.

### Packet Type Discrimination

Modern wire-format UDP packets are distinguished from legacy packets by the first byte:

- Legacy packets: first byte has codec in bits 7–5 (values 0–7, so byte is 0x00–0xFF with specific patterns)
- Protobuf packets: prefixed with a type byte matching `MumbleUDP` message types

The server must support both formats for backward compatibility.

## Audio Routing on the Server

The server does **not** decode or transcode audio. It inspects only the header to determine:

1. **Who sent it** — From the UDP source address / crypto state, or the TCP session.
2. **Where to route it** — From the voice target field.
3. **Who receives it** — Computed from channel membership, links, listeners, and whisper targets.

For each recipient, the server:

1. Adds the sender's session ID (for server→client format).
2. Encrypts with the recipient's AEAD key — OCB2-AES128 (legacy) or AES-256-GCM (secure) — for UDP delivery.
3. Or wraps in a `UDPTunnel` TCP frame (for TCP fallback recipients).

## Bandwidth Enforcement

- Server enforces per-user bandwidth limits.
- Tracked via `BandwidthRecord` — a sliding window of recent audio bytes.
- Users exceeding the limit are suppressed (audio is dropped, not forwarded).
- Server sends `UserState` with `suppress = true` when suppressing.

## Reference

- Protocol reference: `research/mumble/src/MumbleUDP.proto` (reference only; we use native Go encoding)
- Legacy format: `research/gumble/gumble/handlers.go` (`handleUDPTunnel`)
- Varint: `research/gumble/gumble/varint/`
- Audio routing: `research/mumble/src/murmur/Server.cpp` (`processMsg`)
- Receiver grouping: `research/mumble/src/murmur/AudioReceiverBuffer.h`
