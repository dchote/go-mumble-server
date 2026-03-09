# Connection Lifecycle Pattern

> **Status:** Implemented

## Overview

Each Mumble connection progresses through a well-defined sequence of states from initial TCP connect through authentication, state synchronization, steady-state operation, and eventual disconnect. The state machine is the same whether viewed from the server or client side — both use the protocol library's packet framing and handler table infrastructure (`pkg/mumble/protocol`). The server manages one goroutine per connection; a client manages a single connection goroutine.

## State Machine

```
┌───────────┐
│  CONNECT  │  TCP connection accepted
└─────┬─────┘
      │
      ▼
┌───────────┐
│    TLS    │  TLS handshake
│           │  Legacy: TLS 1.2+, client cert optional
│           │  Secure: TLS 1.3 only, client cert required
└─────┬─────┘
      │  failure → close (secure: reject if TLS <1.3 or no client cert)
      ▼
┌───────────┐
│  VERSION  │  Exchange Version messages
│           │  Secure: verify secure-mode capability flag
└─────┬─────┘
      │
      ▼
┌───────────┐
│   CRYPT   │  Server sends CryptSetup (AEAD key + nonces)
│           │  Legacy: 16-byte key (AES-128)
│           │  Secure: 32-byte key (AES-256)
└─────┬─────┘
      │
      ▼
┌───────────┐
│   AUTH    │  Client sends Authenticate; server validates
└─────┬─────┘
      │  reject → send Reject, close
      ▼
┌───────────┐
│   SYNC    │  Server sends ChannelState[], UserState[], ServerConfig
└─────┬─────┘
      │
      ▼
┌───────────┐
│  SYNCED   │  Server sends ServerSync; client is fully connected
└─────┬─────┘
      │
      ▼
┌───────────┐
│  ACTIVE   │  Steady state — control messages + voice
└─────┬─────┘
      │  timeout / disconnect / kick
      ▼
┌───────────┐
│DISCONNECT │  Cleanup, broadcast UserRemove
└───────────┘
```

The security mode affects the TLS handshake, CryptSetup key sizes, and version negotiation. See [protocol/security-modes.md](../protocol/security-modes.md).

## Connection States

| State | Description |
|-------|-------------|
| `Connected` | TCP socket accepted, TLS not yet complete |
| `TLSHandshake` | TLS negotiation in progress |
| `Authenticating` | Awaiting `Authenticate` message from client |
| `Synchronizing` | Sending channel/user state to client |
| `Active` | Fully synchronized, processing messages |
| `Disconnecting` | Connection teardown in progress |

## Implementation Approach

### Goroutine-per-Connection

Each accepted TCP connection spawns a dedicated goroutine that owns the connection's lifecycle. This goroutine:

1. Performs TLS handshake
2. Runs the read loop — reads framed packets, dispatches to handlers
3. Detects disconnect (read error, timeout, or explicit close)
4. Triggers cleanup

A separate write goroutine (or buffered channel) serializes outbound messages to avoid blocking the read loop.

```
                 ┌─────────────────┐
                 │  TCP Listener   │
                 └────────┬────────┘
                          │ Accept()
                          ▼
              ┌───────────────────────┐
              │  Connection Goroutine │
              │                       │
              │  ┌─────────────────┐  │
              │  │   Read Loop     │──┼──► Handler dispatch
              │  └─────────────────┘  │
              │  ┌─────────────────┐  │
              │  │  Write Channel  │◄─┼──── Outbound messages
              │  └────────┬────────┘  │
              │           │           │
              │  ┌────────▼────────┐  │
              │  │  Write Loop     │──┼──► TCP socket
              │  └─────────────────┘  │
              └───────────────────────┘
```

### Timeout Handling

- **Ping interval** — Server and client exchange `Ping` messages periodically (default every 15 seconds).
- **Timeout** — If no data is received from a client within the configured timeout (default 30 seconds), the server disconnects them.
- **Implementation** — Use `SetReadDeadline` on the TCP connection, reset on each received packet.

### Authentication Flow

1. Server receives `Authenticate` with username, password (optional), tokens, and supported codecs.
2. Server checks: username validity (regex), server password, certificate match for registered users, ban list.
3. On failure: send `Reject` with reason (`WrongVersion`, `WrongUserPW`, `WrongServerPW`, `UsernameInUse`, `InvalidUsername`, `NoCertificate`).
4. On success: assign session ID, place user in default/remembered channel, proceed to sync.

### State Synchronization

After authentication, the server sends the full world state:

1. All `ChannelState` messages (depth-first from root). `channel_id` is always emitted (root = 0); root omits the `parent` field (proto2 optional).
2. All `UserState` messages for connected users. `session` and `channel_id` are always emitted (users in root have `channel_id` 0).
3. `ServerConfig` with limits and welcome text
4. `CodecVersion` with negotiated codec
5. `ServerSync` with the client's session ID, welcome text, and root channel permissions

Clients that build the channel tree and user list from these messages will correctly see the root channel and all users, including those in root.

### Disconnect Cleanup (Server)

On disconnect:

1. Remove user from channel
2. Cancel voice targets referencing this user
3. Remove channel listeners
4. Broadcast `UserRemove` to all connected clients
5. Release session ID back to the pool
6. Close TCP connection and UDP crypto state
7. Remove temporary channels if empty

## Client Perspective

A client built on `pkg/mumble/` follows the same state machine but from the opposite side:

1. **Dial** — `tls.Dial()` to the server's TCP port.
2. **Version** — Send `Version`, receive server's `Version`.
3. **CryptSetup** — Receive `CryptSetup`, initialize `CryptState` from `pkg/mumble/crypto` (key size indicates legacy or secure mode).
4. **Authenticate** — Send `Authenticate` with username, password, tokens, and codec support.
5. **Receive state** — Handle `ChannelState`, `UserState`, `ServerConfig`, `CodecVersion` messages to build local state.
6. **ServerSync** — Receive `ServerSync` → transition to active.
7. **Steady state** — Send/receive messages using `protocol.ReadPacket` / `protocol.WriteMessage` (native Go message types). For voice: send UDP pings; if the server echoes them, use UDP for audio; otherwise use TCP tunnel (`UDPTunnel`).
8. **Disconnect** — Close the connection; clean up local state.

The protocol library provides the framing, types, and crypto. The client supplies the connection, handler logic, and local state management.

## Shared Library Components

| Component | Package | Used By |
|-----------|---------|---------|
| Packet read/write | `pkg/mumble/protocol` | Server + Client |
| Message type constants | `pkg/mumble/protocol` | Server + Client |
| Handler table dispatch | `pkg/mumble/protocol` | Server + Client |
| CryptState | `pkg/mumble/crypto` | Server + Client |
| Audio packet parse/build | `pkg/mumble/audio` | Server + Client |
| Core types (Channel, User) | `pkg/mumble` | Server + Client |
| Native Go messages | `pkg/mumble/protocol/messages` | Server + Client |

## Reference

- Murmur: `Server::newClient()`, `Server::connectionClosed()` in `research/mumble/src/murmur/Server.cpp`
- gumble: `Client.readRoutine()` in `research/gumble/gumble/client.go`
