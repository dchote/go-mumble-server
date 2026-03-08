# Technical Overview

> **Status:** Implementation in progress — protocol library complete; server implements core Mumble protocol (connection lifecycle, authentication, channels, users, text messaging, voice routing, ACLs, bans). REST API, web UI, and persistence functional. Full ACL inheritance and advanced features in progress.

go-mumble-server is a native Go implementation of the Mumble voice chat server, built on a reusable protocol library. This document describes the planned architecture, subsystems, and design decisions.

## Technology Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.24+ |
| Protocol serialization | Native Go structs with hand-written wire encoding (no protobuf) |
| Audio codec | Opus (primary), CELT (compatibility) |
| UDP encryption | OCB2-AES128 (legacy mode) / AES-256-GCM (secure mode) |
| TLS | Go standard library `crypto/tls` |
| Database | SQLite (via CGo or pure-Go driver) |
| REST API | Go standard library `net/http` with router |
| API docs | Swagger / OpenAPI 3.0 served at `/docs` |
| Frontend | Vue 3, Vuetify 3, Vite, Vue Router, Vuex |
| Frontend embedding | Go `//go:embed` — frontend dist compiled into binary |
| Configuration | TOML config file + environment variables + flags |
| Logging | Structured logging (`slog`) |

## Library / Server Split

The project is organized into two layers:

1. **Protocol library (`pkg/mumble/`)** — Public Go packages implementing the Mumble protocol. Importable by any Go project. No server dependencies. Suitable for building clients, bots, bridges, monitoring tools, or alternative servers.

2. **Server application (`internal/` + `cmd/`)** — The production Mumble server built on top of the library. Contains server-specific concerns: listener management, audio routing/fan-out, state persistence, REST API, and virtual server orchestration.

```
┌─────────────────────────────────────────────────────────┐
│                    External Go Projects                  │
│             (clients, bots, bridges, tools)              │
└───────────────────────┬─────────────────────────────────┘
                        │ import
┌───────────────────────▼─────────────────────────────────┐
│              pkg/mumble  (Protocol Library)              │
│                                                         │
│  ┌───────────┐ ┌──────────┐ ┌──────────┐ ┌───────────┐ │
│  │ messages  │ │ protocol │ │  crypto  │ │   audio   │ │
│  │ (native   │ │ (framing │ │ (OCB128, │ │ (packets, │ │
│  │  Go + wire)│ │ handler  │ │  AES-GCM │ │  varint,  │ │
│  │           │ │  table)   │ │          │ │  codecs)  │ │
│  └───────────┘ └──────────┘ └──────────┘ └───────────┘ │
│  ┌───────────────────────────────────────────────────┐  │
│  │  Core types: Channel, User, Permission, ACL,      │  │
│  │  VoiceTarget, TextMessage, Version, CryptSetup    │  │
│  └───────────────────────────────────────────────────┘  │
└───────────────────────┬─────────────────────────────────┘
                        │ import
┌───────────────────────▼─────────────────────────────────┐
│              internal/  (Server Application)             │
│                                                         │
│  ┌────────┐ ┌────────┐ ┌──────┐ ┌──────┐ ┌──────────┐ │
│  │ server │ │ audio  │ │ acl  │ │ rest │ │ database │ │
│  │        │ │ router │ │      │ │      │ │          │ │
│  └────────┘ └────────┘ └──────┘ └──────┘ └──────────┘ │
└─────────────────────────────────────────────────────────┘
```

## Architecture

### High-Level Component Diagram

```
┌──────────────────────────────────────────────────────────────┐
│                       go-mumble-server                        │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐ │
│  │  TCP/TLS     │  │  UDP Voice   │  │  REST API + Web UI │ │
│  │  Listener    │  │  Listener    │  │  :9090             │ │
│  │  :64738      │  │  :64738      │  │  /api  /docs  /    │ │
│  └──────┬───────┘  └──────┬───────┘  └─────────┬──────────┘ │
│         │                 │                     │            │
│  ┌──────▼─────────────────▼───────┐   ┌────────▼─────────┐  │
│  │       Protocol Handler         │   │  REST Router     │  │
│  │  (message dispatch by type)    │   │  & SPA Handler   │  │
│  └──────┬─────────────────────────┘   └────────┬─────────┘  │
│         │                                       │            │
│  ┌──────▼───────────────────────────────────────▼─────────┐  │
│  │                     Server Core                        │  │
│  │                                                        │  │
│  │  ┌────────────┐ ┌──────────┐ ┌──────────────────────┐ │  │
│  │  │ Channel    │ │ User     │ │ ACL / Permission     │ │  │
│  │  │ Manager    │ │ Manager  │ │ Evaluator            │ │  │
│  │  └────────────┘ └──────────┘ └──────────────────────┘ │  │
│  │  ┌────────────┐ ┌──────────────────────┐ │  │
│  │  │ Audio      │ │ Ban                  │ │  │
│  │  │ Router     │ │ Manager              │ │  │
│  │  └────────────┘ └──────────┘ └──────────────────────┘ │  │
│  │  ┌────────────┐ ┌──────────┐ ┌──────────────────────┐ │  │
│  │  │ Crypto     │ │ Config   │ │ Virtual Server       │ │  │
│  │  │ State      │ │ Store    │ │ Manager (Meta)       │ │  │
│  │  └────────────┘ └──────────┘ └──────────────────────┘ │  │
│  └──────────────────────────┬─────────────────────────────┘  │
│                             │                                │
│                      ┌──────▼───────┐                        │
│                      │   Database   │                        │
│                      │   (SQLite)   │                        │
│                      └──────────────┘                        │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  Embedded Frontend (//go:embed)                        │  │
│  │  Vue 3 + Vuetify 3 — SPA served from binary           │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

### Network Interfaces

go-mumble-server binds three network interfaces:

1. **Mumble TCP/TLS** (default `:64738`) — Control channel for Mumble protocol messages (native Go encoding). Handles connection setup, authentication, channel/user state synchronization, text messaging, and ACL management.

2. **Mumble UDP** (default `:64738`) — Voice data channel. AEAD-encrypted audio packets (OCB2-AES128 in legacy mode, AES-256-GCM in secure mode). Same port as TCP per Mumble protocol convention.

3. **REST API + Web UI** (default `:9090`) — HTTP management interface with Swagger docs at `/docs` and an embedded Vue 3 + Vuetify management frontend. Used for administration, monitoring, and integration.

### Package Layout

```
go-mumble-server/
├── cmd/
│   └── go-mumble-server/       # Main entry point + frontend embed
│       ├── main.go
│       ├── embed.go             # //go:embed frontend-dist
│       └── frontend-dist/       # Vite build output (git-ignored)
├── frontend/                    # ── Vue 3 + Vuetify Management UI ──
│   ├── src/
│   │   ├── components/          # Reusable UI components
│   │   ├── composables/         # Vue composition API helpers
│   │   ├── layouts/             # App layouts
│   │   ├── pages/               # File-based route pages
│   │   ├── plugins/             # Vuetify, router, store setup
│   │   ├── store/               # Vuex store modules
│   │   ├── styles/              # Theme and global styles
│   │   └── utils/               # API client, formatters
│   ├── index.html
│   ├── package.json
│   └── vite.config.js
├── pkg/
│   └── mumble/                  # ── Public Protocol Library ──
│       ├── protocol/messages/   # Native Go message structs (no protobuf)
│       ├── protocol/            # Packet framing, message type IDs, handler table, wire encoding
│       ├── protocol/wire/       # Hand-written Mumble-compatible wire encoder
│       ├── crypto/              # CryptState: OCB2-AES128 (legacy) + AES-256-GCM (secure)
│       ├── audio/               # Audio packet parsing, varint codec, codec IDs
│       ├── channel.go           # Channel type definition
│       ├── user.go              # User type definition
│       ├── permission.go        # Permission bitmask constants and helpers
│       ├── acl.go               # ACL and Group type definitions
│       ├── voicetarget.go       # VoiceTarget type definition
│       ├── textmessage.go       # TextMessage type definition
│       ├── version.go           # Version encoding/decoding
│       └── ban.go               # BanEntry type definition
├── internal/                    # ── Server-Only Implementation ──
│   ├── server/                  # Virtual server lifecycle, Meta
│   ├── cert/                    # TLS certificate persistence per virtual server
│   ├── mumble/                  # Mumble protocol handler orchestration, per-vserver
│   ├── connection/              # Per-connection state, TLS, CryptState, read loop
│   ├── transport/               # TCP/TLS and UDP listeners
│   ├── handler/                 # REST API handlers
│   ├── audio/                   # Audio routing, fan-out, receiver grouping
│   ├── channel/                 # Channel tree state, linking, listeners
│   ├── user/                    # User session lifecycle
│   ├── auth/                    # Authentication, registration, certificate validation
│   ├── acl/                     # ACL evaluation engine, group resolution, caching
│   ├── ban/                     # Ban list management, autoban
│   ├── config/                  # Configuration loading and validation
│   ├── database/                # SQLite persistence layer
│   ├── discovery/               # mDNS server discovery
│   └── rest/                    # REST API router, SPA serving, middleware
├── api/
│   └── openapi.yaml             # OpenAPI 3.0 specification
├── configs/
│   └── mumble-server.toml       # Default configuration file
├── docs/                        # Project documentation
├── research/                    # Reference implementations
└── go.mod
```

### Library vs Server Boundary

The boundary is drawn by a single question: **does this code need server state?**

| Goes in `pkg/mumble/` | Goes in `internal/` |
|------------------------|---------------------|
| Native Go message structs | TCP/UDP listener management |
| Packet framing (read/write) | Connection accept loops |
| Message type constants | Audio routing and fan-out |
| Handler table infrastructure | Channel tree state management |
| CryptState (encrypt/decrypt, both modes) | ACL evaluation with caching |
| Audio packet parse/build | User session lifecycle |
| Varint codec | Authentication and registration |
| Permission bitmask type | Database persistence |
| Core data types (Channel, User, ACL) | REST API |
| Voice target type definitions | Configuration and logging |
| Version encode/decode | Virtual server orchestration |
| | Embedded frontend + SPA handler |

## Subsystem Design

### Connection Lifecycle

A client connection follows this sequence:

1. **TCP connect** — Client opens a TCP connection to port 64738.
2. **TLS handshake** — Server presents its certificate; optionally verifies client certificate.
3. **Version exchange** — Both sides send `Version` messages.
4. **Crypt setup** — Server sends `CryptSetup` with AEAD key and nonces for UDP encryption (key size depends on security mode).
5. **Authentication** — Client sends `Authenticate` with username, password, tokens, and codec list.
6. **State sync** — Server sends full channel tree (`ChannelState`), all connected users (`UserState`), and server configuration (`ServerConfig`).
7. **Server sync** — Server sends `ServerSync` with the client's session ID, welcome text, and permissions. Client is now fully connected.
8. **Steady state** — Bidirectional protocol messages on TCP; voice on UDP (or tunneled via `UDPTunnel` on TCP).
9. **Disconnect** — TCP close or timeout. Server broadcasts `UserRemove`.

See [patterns/connection-lifecycle-pattern.md](patterns/connection-lifecycle-pattern.md).

### Protocol Handler

Messages are dispatched by their 16-bit type ID using a handler table — an array of handler functions indexed by message type. This mirrors the pattern used in gumble and the original Murmur.

TCP packet framing: 6-byte header (big-endian uint16 type + uint32 length), followed by the message payload (Mumble-compatible wire format, encoded by our hand-written wire layer).

See [protocol/control-messages.md](protocol/control-messages.md) for the full message catalog.

### Audio Router

The audio subsystem handles:

- **Decryption** — AEAD decryption of incoming UDP packets using per-client `CryptState` (OCB2-AES128 in legacy mode, AES-256-GCM in secure mode).
- **Routing** — Determines recipients based on voice target (normal talk, whisper, server loopback).
- **Forwarding** — Sends audio to recipients via UDP (preferred) or TCP tunnel (fallback).

Audio is **not decoded on the server** — packets are forwarded as opaque Opus/CELT frames. The server only inspects the header to determine routing.

Voice targets:
- `0` — Normal talk (current channel + linked channels)
- `1–30` — Whisper targets (configured via `VoiceTarget` messages)
- `31` — Server loopback (echo back to sender)

See [patterns/audio-pipeline-pattern.md](patterns/audio-pipeline-pattern.md).

### Channel Manager

Channels form a tree rooted at a single root channel (ID 0). Each channel has:

- Parent, children, and linked channels
- ACLs and groups (with inheritance)
- Properties: name, description, position, max users, temporary flag
- Active users and channel listeners

See [patterns/channel-tree-pattern.md](patterns/channel-tree-pattern.md).

### ACL Evaluator

Access control uses a layered model, implemented in `internal/acl/evaluator.go`:

1. **Groups** — Named sets of users, defined per-channel with inheritance. Special groups: `all`, `auth`, `in`, `out`, `admin`, `sub`.
2. **ACL entries** — Per-channel rules mapping a user, group, or token to granted/denied permissions. Supports eval-locality (`EvalHere`, ~) and selector inversion (`Invert`, !).
3. **Inheritance** — ACLs and groups cascade down the channel tree unless `InheritACL` is false.
4. **Access tokens** — Clients supply tokens in the Authenticate message; stored on `User.AccessTokens` for token group membership.
5. **Caching** — Permissions cached per (user, channel); invalidated on ACL change (REST PUT) and user channel move.
6. **Default ACLs** — Root channel seeded with `all`, `auth`, `admin` rules via `EnsureDefaultRootACLs()`.

SuperUser (user ID 0) always has Write. UserID is resolved from `registered_users` at authenticate.

Permissions are evaluated as a bitmask. See [patterns/acl-evaluation-pattern.md](patterns/acl-evaluation-pattern.md) and [protocol/permissions.md](protocol/permissions.md).

### Crypto (CryptState)

Each client connection maintains a `CryptState` for UDP encryption. The algorithm depends on the [security mode](protocol/security-modes.md):

- **Legacy mode** — OCB2-AES128. 128-bit key, 3-byte auth tag, single-byte nonce increment. Matches original Mumble.
- **Secure mode** — AES-256-GCM. 256-bit key, 16-byte auth tag, 12-byte explicit nonce. Modern NIST-standard AEAD.

Both modes:
- **Key exchange** — Key + nonces sent via `CryptSetup` over TLS (field sizes vary by mode).
- **Replay protection** — Sliding window to detect replayed or reordered packets.
- **Late/lost tracking** — Statistics for packet loss and late arrivals.

See [protocol/encryption.md](protocol/encryption.md) and [protocol/security-modes.md](protocol/security-modes.md).

### Database

SQLite is the primary storage backend, **encrypted at rest** with AES-256 regardless of protocol security mode. Persisted data includes:

- Registered users and certificates
- Channel tree structure
- ACLs and groups
- Ban lists
- Server configuration
- TLS certificates and keys (per virtual server, for self-signed certs when no config paths are set)
- Logs

Passwords are always stored as Argon2id hashes internally, even when the server runs in legacy mode (which uses PBKDF2 for the wire authentication check).

### REST API

The REST management API runs on a separate HTTP server (default port `9090`):

| Endpoint Group | Description |
|----------------|-------------|
| `GET /health` | Server health and readiness |
| `GET /api/v1/servers` | List virtual servers |
| `GET /api/v1/servers/:id/channels` | Channel tree |
| `GET /api/v1/servers/:id/users` | Connected users |
| `GET /api/v1/servers/:id/bans` | Ban list |
| `GET /api/v1/servers/:id/acl/:channelId` | Channel ACLs |
| `GET /api/v1/servers/:id/config` | Server configuration |
| `GET /docs` | Swagger UI |
| `GET /api/v1/openapi.yaml` | OpenAPI specification |
| `GET /` | Web management UI (SPA) |

All management endpoints require authentication (API key or token-based).

### Web Management Frontend

The server includes an embedded Vue 3 + Vuetify management frontend served on the REST API port.

**Technology:**

| Component | Technology |
|-----------|-----------|
| Framework | Vue 3 (Composition API) |
| UI library | Vuetify 3 (Material Design) |
| Build tool | Vite |
| Routing | Vue Router (file-based via `unplugin-vue-router`) |
| State | Vuex |
| Icons | Material Design Icons (`@mdi/font`) |

**Embedding pattern:**

The frontend is compiled by Vite into `frontend/dist/`, copied to `cmd/go-mumble-server/frontend-dist/`, and embedded into the Go binary using `//go:embed`:

```go
//go:embed frontend-dist
var frontendFS embed.FS

func getFrontendFS() (fs.FS, error) {
    return fs.Sub(frontendFS, "frontend-dist")
}
```

**SPA serving:**

The REST router serves the embedded frontend with SPA-aware fallback:

- Requests for static assets (`.js`, `.css`, `.woff2`, images) that don't exist return 404.
- All other paths that don't match an API route serve `index.html`, allowing Vue Router to handle client-side routing.
- Content-Type headers are set by file extension.

**Development workflow:**

For frontend development with hot reload, the Vite dev server runs separately and proxies API requests to the Go backend:

- Go server: `go run ./cmd/go-mumble-server -frontend-embed=false` (REST API on `:9090`)
- Vite dev: `cd frontend && yarn dev` (UI on `:3000`, proxies `/api`, `/docs`, `/health` to `:9090`)

The `-frontend-embed=false` flag disables SPA serving so the Go server only serves the API, avoiding conflicts with the Vite dev server.

**Build pipeline:**

1. `cd frontend && yarn build` — Vite produces `frontend/dist/` (single bundle via `importMode: 'sync'`)
2. `cp -r frontend/dist cmd/go-mumble-server/frontend-dist` — Copy to embed location
3. `go build ./cmd/go-mumble-server` — `//go:embed` bakes the frontend into the binary

### Configuration

Configuration is loaded from (in order of precedence):

1. Command-line flags
2. Environment variables (`MUMBLE_` prefix)
3. Configuration file (`mumble-server.toml`)
4. Defaults

Key configuration areas:

| Area | Examples |
|------|----------|
| Security | Mode (legacy/secure) |
| Network | Bind address, Mumble port, REST port |
| TLS | Certificate, key, CA |
| Limits | Max users, max bandwidth, message rate limits |
| Channels | Nesting limit, count limit, name regex |
| Users | Default channel, username regex, certificate requirement |
| Database | Path, WAL mode, encryption key |
| Frontend | Embed on/off (`-frontend-embed`) |
| Logging | Level, format, output |

## Documentation Index

### Core

- [Product Overview](product-overview.md) — Project vision and feature summary
- **Technical Overview** — This document
- [Protocol Encoding](architecture/protocol-encoding.md) — Native Go messages, no protobuf (contributors: do not add protobuf)

### Protocol

- [Control Messages](protocol/control-messages.md) — TCP message catalog (types 0–26)
- [Voice Data](protocol/voice-data.md) — UDP audio packet format and routing
- [Security Modes](protocol/security-modes.md) — Legacy vs secure mode design
- [Encryption](protocol/encryption.md) — TLS, AEAD ciphers, password hashing, storage encryption
- [Permissions](protocol/permissions.md) — Permission bitmask definitions

### Patterns

- [Connection Lifecycle](patterns/connection-lifecycle-pattern.md) — Client connect through disconnect
- [Handler Table](patterns/handler-table-pattern.md) — Message dispatch by type ID
- [Channel Tree](patterns/channel-tree-pattern.md) — Hierarchical channel management
- [Audio Pipeline](patterns/audio-pipeline-pattern.md) — Voice routing and forwarding
- [ACL Evaluation](patterns/acl-evaluation-pattern.md) — Permission resolution algorithm
- [Concurrent State](patterns/concurrent-state-pattern.md) — Goroutine-safe shared state

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Go over C++ | Simpler concurrency, single binary, fast compilation, no Qt dependency |
| Legacy + secure modes | Full backward compatibility when needed; modern crypto when possible |
| AES-256-GCM over DTLS | Go has no stdlib DTLS; GCM is NIST standard with hardware accel, same key-exchange model |
| Always-encrypted storage | Local data security should not depend on protocol mode choice |
| Protocol library in `pkg/` | Enables reuse for clients, bots, bridges, and tools without importing server code |
| Goroutine-per-connection | Natural fit for Go; avoids complex thread pool / event loop |
| SQLite for persistence | Zero-config, embedded, sufficient for Mumble server workloads |
| Separate REST port | Clean separation between Mumble protocol traffic and management |
| Swagger at `/docs` | Self-documenting API; standard tooling for client generation |
| TOML configuration | Human-friendly, well-supported in Go ecosystem |
| `internal/` for server logic | Enforces encapsulation; public API only via `pkg/mumble/` and REST |
| Core types in library | `Channel`, `User`, `Permission`, `ACL` live in `pkg/` so clients have the same vocabulary as the server |
| **No Google protobuf** | Protocol uses native Go structs and hand-written wire encoding. Do not add `google.golang.org/protobuf` or protoc-generated code. |
| Vue 3 + Vuetify frontend | Material Design UI with rich component library; Vuetify provides accessible, responsive components out of the box |
| Embedded frontend via `//go:embed` | Single binary deployment; no separate web server needed; same binary serves both API and UI |
| Vite with single-bundle build | `importMode: 'sync'` produces a single JS bundle, avoiding chunk 404 issues when served by the Go SPA handler |
| Separate dev server for frontend | Hot module replacement during development; Vite proxies API calls to the Go backend |

## Reference Implementations

| Resource | Path / URL |
|----------|-----------|
| Mumble (Murmur) server | `research/mumble/src/murmur/` |
| Mumble protocol reference | `research/mumble/src/Mumble.proto`, `research/mumble/src/MumbleUDP.proto` (reference only; we use native Go, not protobuf) |
| Mumble server config reference | `research/mumble/auxiliary_files/mumble-server.ini` |
| gumble Go client library | `research/gumble/` |
| Mumble protocol documentation | `research/mumble/docs/dev/network-protocol/` |
