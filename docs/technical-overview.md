# Technical Overview

> **Status:** Implementation in progress — protocol library complete; server implements core Mumble protocol (connection lifecycle, authentication, channels, users, text messaging, voice routing, ACLs, bans). REST API, web UI, and persistence functional. Full ACL inheritance and advanced features in progress.

go-mumble-server is a native Go implementation of the Mumble voice chat server, built on a reusable protocol library. This document describes the planned architecture, subsystems, and design decisions.

## Technology Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.25+ |
| Protocol serialization | Native Go structs with hand-written wire encoding (no protobuf) |
| Audio codec | Opus (primary), CELT (compatibility) |
| UDP encryption | OCB2-AES128 (legacy mode) / AES-256-GCM (secure mode) |
| TLS | Go standard library `crypto/tls` |
| Database | SQLite (via CGo or pure-Go driver) |
| REST API | Go standard library `net/http` with router |
| API docs | Swagger / OpenAPI 3.0 served at `/docs` |
| Frontend | Vue 3, Vuetify 3, Vite, Vue Router, Vuex |
| Frontend embedding | Go `//go:embed` — frontend dist compiled into binary |
| Configuration | TOML bootstrap + SQLite (runtime, editable via REST API and web UI) |
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
│  │  Listener    │  │  Listener    │  │  :64730            │ │
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

2. **Mumble UDP** (default `:64738`) — Voice data channel. AEAD-encrypted audio packets (OCB2-AES128 in legacy mode, AES-256-GCM in secure mode). Same port as TCP per Mumble protocol convention. The server echoes UDP pings so clients can confirm connectivity before using UDP for voice.

3. **REST API + Web UI** (default `:64730`) — HTTP management interface with Swagger docs at `/docs` and an embedded Vue 3 + Vuetify management frontend. Used for administration, monitoring, and integration.

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
│   ├── config/                  # Bootstrap (TOML) + DB config loading
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
4. **Crypt setup** — Server sends `CryptSetup` with AEAD key and nonces for UDP encryption (key size signals negotiated crypto tier).
5. **Authentication** — Client sends `Authenticate` with username, password, tokens, and codec list.
6. **State sync** — Server sends full channel tree (`ChannelState`), all connected users (`UserState`), and server configuration (`ServerConfig`).
7. **Server sync** — Server sends `ServerSync` with the client's session ID, welcome text, and permissions. Client is now fully connected.
8. **Steady state** — Bidirectional protocol messages on TCP; voice on UDP (after clients receive ping echo) or tunneled via `UDPTunnel` on TCP when UDP is unavailable.
9. **Disconnect** — TCP close or timeout. Server broadcasts `UserRemove`.

See [patterns/connection-lifecycle-pattern.md](patterns/connection-lifecycle-pattern.md).

### Protocol Handler

Messages are dispatched by their 16-bit type ID using a handler table — an array of handler functions indexed by message type. This mirrors the pattern used in gumble and the original Murmur.

TCP packet framing: 6-byte header (big-endian uint16 type + uint32 length), followed by the message payload (Mumble-compatible wire format, encoded by our hand-written wire layer).

See [protocol/control-messages.md](protocol/control-messages.md) for the full message catalog.

### Audio Router

The audio subsystem handles:

- **UDP ping echo** — Clients send encrypted UDP pings (codec type 1) to test connectivity. The server echoes them back; without this, clients assume UDP is blocked and fall back to TCP tunneling.
- **Decryption** — AEAD decryption of incoming UDP packets using per-client `CryptState` (OCB2-AES128 in legacy mode, AES-256-GCM in secure mode).
- **Packet rewriting** — Client→server packets omit the sender's session ID; server→client packets must include it. The server inserts the sender session ID (varint) between the header and the rest of the payload before forwarding.
- **Routing** — Determines recipients based on voice target (normal talk, whisper, server loopback).
- **Forwarding** — Sends audio to recipients via UDP (if the recipient has sent at least one UDP packet) or TCP tunnel (fallback via `UDPTunnel` message).

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
6. **Default ACLs** — Root channel seeded with `all` (Traverse, Enter, Speak, Whisper, TextMessage, Listen), `auth` (MakeTempChannel, SelfRegister), `admin` (Write) via `EnsureDefaultRootACLs()`.

UserID is resolved from `registered_users` or API users at authenticate. Unregistered users receive userID 0 and get permissions through normal ACL evaluation. API users (from the management `users` table) receive synthetic userIDs and RBAC roles are resolved for `@admin` membership. See [RBAC Strategy](patterns/acl-evaluation-pattern.md#rbac-strategy-api-users).

Permissions are evaluated as a bitmask. See [patterns/acl-evaluation-pattern.md](patterns/acl-evaluation-pattern.md) and [protocol/permissions.md](protocol/permissions.md).

### Crypto (CryptState)

Each client connection maintains a `CryptState` for UDP encryption. The algorithm is [negotiated per client](protocol/security-modes.md):

- **Legacy mode** — OCB2-AES128. 128-bit key, 3-byte auth tag, single-byte nonce increment. Matches original Mumble.
- **Secure mode** — AES-256-GCM. 256-bit key, 16-byte auth tag, 12-byte explicit nonce. Modern NIST-standard AEAD.

Both modes:
- **Key exchange** — Key + nonces sent via `CryptSetup` over TLS (field sizes vary by mode).
- **Replay protection** — Sliding window to detect replayed or reordered packets.
- **Late/lost tracking** — Statistics for packet loss and late arrivals.

See [protocol/encryption.md](protocol/encryption.md) and [protocol/security-modes.md](protocol/security-modes.md).

### Database

SQLite is the primary storage backend. Tables include:

| Table | Description |
|-------|-------------|
| `meta_config` | Global process-level settings (single row) |
| `server_configs` | Per-virtual-server configuration |
| `servers` | Virtual server definitions |
| `channels` | Channel tree structure (root = ID 0) |
| `channel_acls` | Per-channel ACL entries |
| `channel_groups` | Per-channel group definitions |
| `registered_users` | Registered user accounts and certificates |
| `bans` | Server ban list |
| `tls_certs` | Per-virtual-server TLS certificates and keys |

Management (API) user passwords are stored as bcrypt hashes; Mumble registered-user passwords use Argon2id. The wire protocol in legacy mode may use different algorithms for authentication; storage hashing is independent.

### REST API

The REST management API runs on a separate HTTP server (default port `64730`, configurable via `rest_port` in config or `MUMBLE_REST_PORT`):

| Endpoint Group | Methods | Description |
|----------------|---------|-------------|
| `/health` | GET | Server health and readiness |
| `/api/v1/servers` | GET, POST | Virtual server CRUD |
| `/api/v1/servers/:id` | GET, PATCH, DELETE | Single virtual server |
| `/api/v1/servers/:id/channels` | GET, POST | Channel tree + create |
| `/api/v1/servers/:id/channels/:channelId` | PATCH, DELETE | Update/delete channel |
| `/api/v1/servers/:id/channels/:channelId/acl` | GET, PUT | Channel ACLs and groups |
| `/api/v1/servers/:id/users` | GET | Connected Mumble users; admin receives full data; non-admin receives sanitized list (no `address`, `certificate_hash`) |
| `/api/v1/servers/:id/users/:sessionId/kick` | POST | Kick connected user |
| `/api/v1/servers/:id/users/:sessionId/mute` | POST | Mute/unmute connected user |
| `/api/v1/servers/:id/users/:sessionId/ban` | POST | Ban and kick connected user |
| `/api/v1/servers/:id/registered-users` | GET, POST | Registered user CRUD |
| `/api/v1/servers/:id/registered-users/:userId` | PATCH, DELETE | Single registered user |
| `/api/v1/servers/:id/bans` | GET, POST | Ban list CRUD |
| `/api/v1/servers/:id/bans/:banId` | DELETE | Remove a ban |
| `/api/v1/servers/:id/config` | GET, PATCH | Per-server configuration |
| `/api/v1/meta/config` | GET, PATCH | Global (meta) configuration |
| `/docs` | GET | Swagger UI |
| `/api/v1/openapi.yaml` | GET | OpenAPI specification |
| `/` | GET | Web management UI (SPA) |

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

**Patterns and guidelines:**

- [Frontend Guide](patterns/frontend-guide.md) — Vue/Vuetify patterns, routing, API usage
- [UI Style Guidelines](patterns/ui-style-guidelines.md) — Layout, spacing, tables, forms, Vuetify best practices

**Development workflow:**

For frontend development with hot reload, the Vite dev server runs separately and proxies API requests to the Go backend:

- Go server: `go run ./cmd/go-mumble-server -frontend-embed=false` (REST API on `:64730`)
- Vite dev: `cd frontend && yarn dev` (UI on `:3000`, proxies `/api`, `/docs`, `/health` to `:64730`)

The `-frontend-embed=false` flag disables SPA serving so the Go server only serves the API, avoiding conflicts with the Vite dev server.

**Build pipeline:**

1. `cd frontend && yarn build` — Vite produces `frontend/dist/` (single bundle via `importMode: 'sync'`)
2. `cp -r frontend/dist cmd/go-mumble-server/frontend-dist` — Copy to embed location
3. `go build ./cmd/go-mumble-server` — `//go:embed` bakes the frontend into the binary

### Configuration

Configuration uses a **two-tier** model:

**Tier 1 — Bootstrap** (TOML file / environment variables / flags):

Process-level settings needed before the database is open. Loaded with precedence: flags > env vars (`MUMBLE_` prefix) > TOML file > defaults.

| Setting | Description |
|---------|-------------|
| `database.path` | SQLite database file path |
| `tls.cert`, `tls.key` | TLS certificate and key paths (PEM) |
| `logging.level` | Log level (`debug`, `info`, `warn`, `error`) |
| `frontend-embed` (flag only) | Embed the web management UI |

The TOML file (`configs/mumble-server.toml`) also contains initial values for database-backed settings, which seed the database on first run only.

**Tier 2 — Database** (SQLite, editable via REST API and web UI):

All runtime settings are stored in SQLite. Two tables:

| Table | Scope | REST Endpoint | Key Settings |
|-------|-------|---------------|--------------|
| `meta_config` | Global (process-level) | `GET/PATCH /api/v1/meta/config` | Bind address, ports, Bonjour, JWT |
| `server_configs` | Per-virtual-server | `GET/PATCH /api/v1/servers/:id/config` | Max users, bandwidth, welcome text, password, default channel, cert required, channel limits |

On first start, the TOML/env/flag values seed both tables. Subsequent changes are made through the API or web UI and persist in the database.

## Documentation Index

### Core

- [Product Overview](product-overview.md) — Project vision and feature summary
- **Technical Overview** — This document
- [Protocol Encoding](architecture/protocol-encoding.md) — Native Go messages, no protobuf (contributors: do not add protobuf)

### Protocol

- [Control Messages](protocol/control-messages.md) — TCP message catalog (types 0–26)
- [Voice Data](protocol/voice-data.md) — UDP audio packet format and routing
- [Security Modes](protocol/security-modes.md) — Per-client negotiated crypto tiers
- [Encryption](protocol/encryption.md) — TLS, AEAD ciphers, password hashing
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
| SQLite not encrypted at rest | Current implementation uses plain SQLite; use filesystem or deployment encryption if required |
| Protocol library in `pkg/` | Enables reuse for clients, bots, bridges, and tools without importing server code |
| Goroutine-per-connection | Natural fit for Go; avoids complex thread pool / event loop |
| SQLite for persistence | Zero-config, embedded, sufficient for Mumble server workloads |
| Separate REST port | Clean separation between Mumble protocol traffic and management |
| Swagger at `/docs` | Self-documenting API; standard tooling for client generation |
| TOML bootstrap + SQLite config | TOML for pre-DB settings; SQLite for runtime config editable via API/UI |
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
