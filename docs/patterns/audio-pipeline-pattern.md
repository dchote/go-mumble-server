# Audio Pipeline Pattern

> **Status:** Design

## Overview

The audio pipeline is the performance-critical path in a Mumble server. Audio packets arrive via UDP (or TCP tunnel), are decrypted, routed to recipients based on voice targets and channel topology, and forwarded without re-encoding. The server never decodes audio — it operates on opaque codec frames.

## Audio Packet Flow

```
  Client A (sender)
       │
       │  UDP (AEAD encrypted: OCB2 legacy / GCM secure)
       │  ─── or ───
       │  TCP tunnel (UDPTunnel, type 1)
       ▼
┌──────────────┐
│   Receive    │  Decrypt (UDP) or extract (TCP tunnel)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│  Parse Header│  Extract: codec, target, session, sequence
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Route / Fan  │  Determine recipients from voice target
│    Out       │  + channel links + listeners
└──────┬───────┘
       │
       ▼ (for each recipient)
┌──────────────┐
│   Forward    │  Re-encrypt for recipient (UDP)
│              │  or wrap in UDPTunnel (TCP)
└──────────────┘
       │
       ▼
  Client B, C, ... (recipients)
```

## Packet Format (Legacy Binary)

The original audio packet format (used inside `UDPTunnel` and legacy UDP):

```
Byte 0:       ┌─────────┬────────┐
              │ Codec   │ Target │
              │ (3 bit) │ (5 bit)│
              └─────────┴────────┘
Varint:       Sender Session (server→client only)
Varint:       Sequence Number
Varint:       Payload length (bit 13 = terminator flag)
Bytes:        Opus/CELT frame data
Optional:     3× float32 positional audio (X, Y, Z)
```

Codec IDs:
- `0` — CELT Alpha
- `2` — Speex (deprecated)
- `3` — CELT Beta
- `4` — Opus (preferred)

## Packet Format (Protobuf UDP — since protocol 1.5)

Modern clients may use protobuf-encoded UDP packets (`MumbleUDP.proto`):

```protobuf
message Audio {
    uint32 target = 1;           // voice target (0 = normal, 1-30 = whisper, 31 = loopback)
    uint32 context = 2;          // 0 = normal, 1 = shout
    uint32 sender_session = 3;   // set by server
    uint64 frame_number = 4;
    bytes  opus_data = 5;
    float  positional_data = 6;  // repeated, 3 floats
    float  volume_adjustment = 7;
    bool   is_terminator = 8;
}
```

## Voice Targets

| Target | Meaning |
|--------|---------|
| 0 | Normal — send to current channel + linked channels |
| 1–30 | Whisper — send to preconfigured `VoiceTarget` |
| 31 | Server loopback — echo back to sender |

Whisper targets are configured per-client via `VoiceTarget` protobuf messages and can specify:
- Specific user sessions
- A channel (with options: links, children, group filter)

## Routing Algorithm

For a voice packet from user A with target T:

1. **Target 31 (loopback):** Return packet to sender A.
2. **Target 0 (normal):**
   - Collect all users in A's channel.
   - Collect all users in channels linked to A's channel.
   - Collect all listeners on A's channel.
   - Remove A from the recipient set.
   - Filter by `Speak` permission and deaf/mute state.
3. **Target 1–30 (whisper):**
   - Look up A's `VoiceTarget[T]`.
   - Resolve target sessions, channels (optionally with links/children/group).
   - Filter by permission and state.

## Receiver Grouping

Recipients are grouped by transport, protocol version, and security mode to minimize work:

```
Recipients
├── UDP recipients (all share the same security mode)
│   ├── Legacy format group → AEAD encrypt + send per recipient
│   └── Protobuf format group → AEAD encrypt + send per recipient
└── TCP recipients
    ├── Legacy format group → wrap in UDPTunnel + send
    └── Protobuf format group → wrap in UDPTunnel + send
```

The `AudioReceiverBuffer` pattern (from Murmur) groups recipients that share the same context, version, and volume adjustment to avoid redundant packet construction. In secure mode, the per-recipient encryption uses AES-256-GCM with 28 bytes overhead instead of OCB2's 4 bytes — audio packet construction accounts for this difference.

## Performance Considerations

- **Zero-copy forwarding** — Avoid decoding/re-encoding audio. Forward opaque frames.
- **Goroutine for UDP** — A dedicated goroutine reads UDP packets and handles audio routing, minimizing latency.
- **Minimal locking** — Audio routing needs channel links and user lists, which are read-locked (RWMutex) and rarely mutated during steady-state operation.
- **Bandwidth enforcement** — Per-user bandwidth tracking via a leaky bucket. Excessive senders are suppressed.
- **Batch sends** — Where possible, batch UDP `sendto` calls for multiple recipients.

## Codec Negotiation

On connect and when the user population changes:

1. Server collects codec support from all connected clients.
2. Server selects the codec supported by the most clients (Opus preferred).
3. Server broadcasts `CodecVersion` if the preferred codec changes.

## Reference

- Murmur audio routing: `Server::processMsg()` in `research/mumble/src/murmur/Server.cpp`
- Murmur receiver buffer: `AudioReceiverBuffer` in `research/mumble/src/murmur/AudioReceiverBuffer.h`
- gumble audio handling: `handleUDPTunnel()` in `research/gumble/gumble/handlers.go`
- gumble varint: `research/gumble/gumble/varint/`
