# Product Overview

**go-mumble-server** is a modern, from-scratch implementation of the Mumble voice chat server written in Go. It implements the full Mumble protocol and is wire-compatible with all existing Mumble clients — any standard Mumble client can connect and use it as a drop-in replacement for the original Murmur server.

**Status: Beta** — Functionally complete; v0.1 release ready.

The project is structured as two layers: a **reusable Mumble protocol library** (`pkg/`) that can be imported by any Go project, and a **production server** built on top of it. The library provides everything needed to implement Mumble clients, bots, bridges, or alternative servers in Go.

## Motivation

The original Mumble server (Murmur) is a mature C++/Qt application that has served the community well for over a decade. However, its codebase carries significant legacy weight: deep Qt framework coupling, complex threading via Qt event loops, and a build process tied to C++ toolchains. The existing Go client library (gumble) is unmaintained and client-only by design. **go-mumble-server** re-imagines both the server and the protocol layer with modern priorities:

- **Simplicity** — A single, statically-linked binary with zero runtime dependencies. No Qt, no C++ toolchain, no shared library management.
- **Reusable protocol library** — The Mumble protocol implementation lives in importable `pkg/` packages. Build clients, bots, monitoring tools, or entirely new servers on the same foundation.
- **Operational clarity** — Structured logging, health endpoints, and a REST management API with Swagger documentation out of the box.
- **Deployment flexibility** — Runs anywhere Go runs. Trivial containerization. Native cross-compilation for Linux, macOS, Windows, and ARM targets. The repository can also be added as a Home Assistant add-on repository for one-click install; see the README and addon documentation.
- **Maintainability** — Go's straightforward concurrency model (goroutines and channels) replaces the original's intricate mutex hierarchy and Qt signal/slot threading.

## Two-Layer Architecture

### Protocol Library (`pkg/`)

The public Go packages provide the building blocks for any Mumble protocol implementation:

- **`pkg/mumble`** — Core types: `Channel`, `User`, `Permission`, `ACL`, `VoiceTarget`, `TextMessage`. Shared by both client and server code.
- **`pkg/mumble/protocol/messages`** — Native Go structs for all Mumble control messages and UDP audio messages (no protobuf).
- **`pkg/mumble/protocol`** — Packet framing (read/write with the 6-byte TCP header), message type constants, handler table infrastructure, and varint codec for audio packets.
- **`pkg/mumble/crypto`** — `CryptState` for UDP voice encryption/decryption. Supports both OCB2-AES128 (legacy) and AES-256-GCM (secure) modes.
- **`pkg/mumble/audio`** — Audio packet parsing, voice target resolution types, and codec negotiation constants.

These packages have no dependency on the server — they are pure protocol primitives.

### Server Application (`internal/` + `cmd/`)

The server is built on top of the protocol library and adds:

- TCP/TLS and UDP listener management
- Audio routing and fan-out
- Channel tree state management with ACL evaluation
- User session lifecycle and authentication
- SQLite persistence
- REST management API with Swagger
- Vue 3 + Vuetify web management UI (embedded in binary)
- Virtual server support
- Configuration, logging, and operational tooling

## Security Modes

go-mumble-server negotiates security **per client** during the Version exchange. Three tiers are supported:

- **Legacy** (default) — 100% backward compatible with all existing Mumble clients. OCB2-AES128 for UDP voice, TLS 1.2+. This is the default mode for any client that does not advertise crypto capabilities.

- **Secure** — Modern cryptography for clients that support it. AES-256-GCM for UDP voice, TLS 1.3, mandatory client certificates. Clients advertise this capability; the server upgrades when both support it.

- **Lite** — No UDP encryption (cleartext voice) for constrained devices (e.g. ESP32) on trusted networks. Control channel remains TLS-encrypted.

Standard Mumble clients omit capability negotiation and default to legacy. Mixed client populations are supported: a desktop client may use legacy while a secure-aware client uses secure on the same server.

**Mixed-mode channel enforcement** — When clients with different crypto modes share a channel, the server forces all audio in that channel through TCP tunnel relay (UDPTunnel over the TLS-encrypted control connection). This ensures each client's security level is maintained correctly. When the channel returns to a single mode, UDP transport is re-enabled automatically.

**Password storage** — Management UI (API) user passwords are hashed with bcrypt. Mumble registered-user passwords are stored as Argon2id hashes. SQLite storage is not encrypted at rest in the current implementation; rely on filesystem or deployment-level encryption if required.

See [protocol/security-modes.md](protocol/security-modes.md) for the full design.

## Core Functionality

go-mumble-server implements the complete Mumble server feature set:

- **Voice communication** — Low-latency Opus audio with UDP transport and automatic TCP fallback. Positional audio support.
- **Channel hierarchy** — Full channel tree with sub-channels, linking, and temporary channels.
- **Access control** — ACL and group-based permissions with inheritance, per-channel overrides, and access tokens.
- **Text messaging** — Channel messages, private messages, and tree-wide broadcasts.
- **User management** — Certificate-based authentication, user registration, and server passwords.
- **Whisper / voice targets** — Directed audio to specific users, channels, or ACL groups.
- **Channel listeners** — Users can listen to channels without joining them.
- **Server configuration** — Bandwidth limits, rate limiting, user limits, channel constraints, and welcome messages.
- **Encryption** — TLS for control; UDP voice uses AEAD (OCB2-AES128 legacy, AES-256-GCM secure) or cleartext (lite) per client.
- **Virtual servers** — Multiple logical servers within a single process.

## Web Management UI

go-mumble-server ships with a Vue 3 + Vuetify management frontend embedded directly into the server binary. Open `http://localhost:64730` to access:

- Server status dashboard and health monitoring
- Channel tree visualization and management
- Connected user list with session details
- ACL and group editor
- Ban list management
- Server configuration
- Virtual server controls

The frontend is built with Vite and embedded via Go's `//go:embed` — no separate web server or static file hosting required. It can be disabled with `-frontend-embed=false` for headless deployments.

## REST Management API

The web UI is a consumer of the REST API, which is also available for direct integration. The REST API runs on port `64730` by default (configurable via `rest_port` in config or `MUMBLE_REST_PORT`):

- Server status, health, and statistics
- Channel and user management
- ACL and group configuration
- Ban list management
- Virtual server lifecycle control
- Server configuration

Interactive API documentation is served at `/docs` via Swagger UI.

## Protocol Compatibility

go-mumble-server targets full compatibility with the Mumble protocol as defined by the upstream project:

- **Control channel** — TCP with TLS, Mumble protocol messages (27 message types, native Go encoding)
- **Voice channel** — UDP with AEAD encryption, or tunneled over TCP
- **Per-client negotiation** — Legacy (default), secure, or lite; standard Mumble clients use legacy automatically
- **Version negotiation** — Supports protocol version exchange and codec negotiation (Opus preferred, CELT fallback)
- **Proto2 field presence** — Outgoing `UserState` voice flags encode explicit `false` values so echo-driven clients (Mumla, Plumble) can unmute; mute/deaf cascade matches murmur ([0007](features/0007-userstate-field-presence.md))

## Target Users

- **Self-hosters** who want a Mumble server that is trivial to deploy and operate
- **Gaming communities** looking for low-latency, high-quality voice chat they control
- **Organizations** needing a private, on-premises voice communication server
- **Developers** building tools and integrations around Mumble via the REST API
- **Go developers** building Mumble clients, bots, bridges, or monitoring tools using the protocol library

## License

MIT License — Copyright (c) 2026 Daniel Chote
