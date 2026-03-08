# go-mumble-server

A modern, from-scratch implementation of the [Mumble](https://www.mumble.info/) voice chat server written in Go. Wire-compatible with all standard Mumble clients — drop-in replacement for the original Murmur server.

The Mumble protocol implementation is a **reusable Go library** (`pkg/mumble/`) that can be imported independently to build clients, bots, bridges, or other tools.

## Overview

go-mumble-server re-imagines the Mumble server with modern priorities: a single static binary, zero runtime dependencies, built-in REST management API, and Go's straightforward concurrency model replacing the original's C++/Qt complexity.

**Mumble protocol on TCP/TLS :64738** — Full control channel with protobuf messages.
**Voice on UDP :64738** — Low-latency AEAD-encrypted audio with TCP tunnel fallback.
**REST management API on :9090** — Administration and monitoring with Swagger docs at `/docs`.
**Web management UI** — Vue 3 + Vuetify frontend embedded in the binary, served alongside the REST API.

Two **security modes**: **legacy** (100% backward compatible with all Mumble clients) and **secure** (modern crypto, breaks backward compatibility). Local storage is always encrypted regardless of mode.

## Features

### Server

- **Full Mumble protocol** — All 27 control message types, UDP and TCP voice transport
- **Opus audio** — Preferred codec with CELT fallback for legacy clients
- **Channel hierarchy** — Tree structure with linking, temporary channels, and channel listeners
- **ACL permissions** — Group-based access control with inheritance, tokens, and per-channel overrides
- **Text messaging** — Private, channel, and tree-wide messages with HTML support
- **Whisper / voice targets** — Directed audio to specific users, channels, or groups
- **User management** — Certificate-based identity, registration, and server passwords
- **Virtual servers** — Multiple logical servers in a single process
- **Dual security modes** — Legacy (OCB2-AES128, TLS 1.2+) for compatibility; Secure (AES-256-GCM, TLS 1.3, Argon2id) for modern security
- **Encrypted storage** — AES-256 encrypted database at rest, Argon2id password hashes, regardless of protocol mode
- **REST API** — Server management, monitoring, and integration with Swagger UI at `/docs`
- **Web management UI** — Vue 3 + Vuetify frontend embedded in the server binary
- **SQLite storage** — Zero-config persistence for users, channels, ACLs, and bans

### Protocol Library

- **Importable as a Go module** — `import "github.com/user/go-mumble-server/pkg/mumble"`
- **Protobuf types** — Generated types for all 27 control messages and UDP audio messages
- **Packet framing** — Read/write functions for the 6-byte TCP header format
- **Handler table** — Message dispatch infrastructure usable by both server and client code
- **CryptState** — AEAD encrypt/decrypt for UDP voice packets (OCB2-AES128 legacy, AES-256-GCM secure)
- **Audio packets** — Parse/build audio packets with varint codec, codec IDs, voice targets
- **Core types** — `Channel`, `User`, `Permission`, `ACL`, `VoiceTarget`, `TextMessage`, `BanEntry`
- **No server dependencies** — Pure protocol primitives with zero coupling to server internals

## Requirements

- Go 1.24+
- Node.js 20+ and Yarn (for frontend development)

## Building

### Full build (frontend + server)

```bash
./scripts/build.sh
```

This builds the Vue frontend, copies the dist into the Go embed location, and compiles the server binary to `build/go-mumble-server`. The frontend is embedded in the binary via `//go:embed`.

### Server only (skip frontend)

```bash
SKIP_FRONTEND=true ./scripts/build.sh
```

Or directly:

```bash
go build -o go-mumble-server ./cmd/go-mumble-server
```

## Running

```bash
# Start with defaults (legacy mode, Mumble on :64738, REST on :9090)
./go-mumble-server

# Start in secure mode
./go-mumble-server -security-mode secure

# With configuration file
./go-mumble-server -config /path/to/mumble-server.toml

# With environment variables
MUMBLE_SECURITY_MODE=secure MUMBLE_PORT=64738 MUMBLE_REST_PORT=9090 ./go-mumble-server
```

## Configuration

Configuration is loaded from (in order of precedence): command-line flags, environment variables (`MUMBLE_` prefix), configuration file, defaults.

| Setting | Default | Description |
|---------|---------|-------------|
| `port` | 64738 | Mumble protocol port (TCP + UDP) |
| `rest-port` | 9090 | REST API + web UI port |
| `frontend-embed` | true | Serve embedded web UI on the REST port |
| `host` | 0.0.0.0 | Bind address |
| `database` | mumble-server.sqlite | SQLite database path |
| `security-mode` | legacy | Security mode: `legacy` or `secure` |
| `ssl-cert` | | TLS certificate (PEM) |
| `ssl-key` | | TLS private key (PEM) |
| `max-users` | 100 | Maximum concurrent users |
| `max-bandwidth` | 72000 | Maximum bandwidth per user (bps) |
| `welcome-text` | | Server welcome message (HTML) |
| `server-password` | | Server-wide password |
| `log-level` | info | Logging level (debug, info, warn, error) |

See [docs/technical-overview.md](docs/technical-overview.md) for the full configuration reference.

## Web Management UI

The server includes a Vue 3 + Vuetify management frontend that is embedded into the Go binary and served on the REST API port (default `:9090`). Open `http://localhost:9090` in a browser to access the management interface.

Features: server status dashboard, channel tree management, connected user list, ACL editor, ban list management, server configuration, and virtual server controls.

The frontend can be disabled at runtime with `-frontend-embed=false` for headless/API-only deployments.

## REST API

The management API runs on port 9090 by default. Interactive Swagger documentation is available at `/docs`. The web UI is a consumer of this same REST API.

```bash
# Server health
curl http://localhost:9090/health

# List connected users
curl http://localhost:9090/api/v1/servers/1/users

# Channel tree
curl http://localhost:9090/api/v1/servers/1/channels
```

## Using the Protocol Library

The `pkg/mumble/` packages can be imported by any Go project to build Mumble clients, bots, or tools:

```go
import (
    "github.com/user/go-mumble-server/pkg/mumble"
    "github.com/user/go-mumble-server/pkg/mumble/proto"
    "github.com/user/go-mumble-server/pkg/mumble/protocol"
    "github.com/user/go-mumble-server/pkg/mumble/crypto"
    "github.com/user/go-mumble-server/pkg/mumble/audio"
)
```

Example — connecting to a Mumble server and reading packets:

```go
conn, _ := tls.Dial("tcp", "localhost:64738", &tls.Config{
    InsecureSkipVerify: true,
})

// Send version
protocol.WriteProto(conn, protocol.MessageVersion, &proto.Version{
    Release: stringPtr("MyBot 1.0"),
    Opus:    boolPtr(true),
})

// Read loop using the library's packet framing
for {
    msgType, payload, err := protocol.ReadPacket(conn)
    if err != nil {
        break
    }
    handlers.Dispatch(msgType, payload)
}
```

The library handles framing, protobuf serialization, CryptState for UDP, audio packet parsing, and provides all the core Mumble types. Your code provides the connection management and handler logic.

See [docs/technical-overview.md](docs/technical-overview.md) for the full package layout and the library/server boundary.

## Local Development

For frontend development with hot reload, run the Vite dev server separately from the Go backend.

**Terminal 1** — Start the Go server (API only, no embedded frontend):

```bash
go run ./cmd/go-mumble-server -frontend-embed=false
```

**Terminal 2** — Start the Vite dev server (frontend with hot reload):

```bash
cd frontend && yarn dev
```

Open [http://localhost:3000](http://localhost:3000). The Vite dev server proxies `/api`, `/docs`, and `/health` to the Go server at `http://localhost:9090`.

To use a different backend URL:

```bash
VITE_API_PROXY_TARGET=http://localhost:9090 yarn dev
```

## Project Structure

```
go-mumble-server/
├── cmd/
│   └── go-mumble-server/       # Main entry point + frontend embed
│       ├── main.go
│       ├── embed.go             # //go:embed frontend-dist
│       └── frontend-dist/       # Vite build output (copied by build script)
├── frontend/                    # ── Vue 3 + Vuetify Management UI ──
│   ├── src/
│   │   ├── components/          # Reusable UI components
│   │   ├── composables/         # Vue composition API helpers
│   │   ├── layouts/             # App layouts (authenticated, default)
│   │   ├── pages/               # File-based route pages
│   │   ├── plugins/             # Vuetify, router, store setup
│   │   ├── store/               # Vuex store modules
│   │   ├── styles/              # Theme and global styles
│   │   ├── utils/               # API client, formatters
│   │   ├── App.vue
│   │   └── main.js
│   ├── index.html
│   ├── package.json
│   ├── vite.config.js
│   └── yarn.lock
├── pkg/
│   └── mumble/                  # ── Public Protocol Library ──
│       ├── proto/               # Generated protobuf types
│       ├── protocol/            # Packet framing, message types, handler table
│       ├── crypto/              # CryptState (legacy + secure modes)
│       ├── audio/               # Audio packets, varint, codec IDs
│       ├── channel.go           # Channel type
│       ├── user.go              # User type
│       ├── permission.go        # Permission bitmask
│       ├── acl.go               # ACL / Group types
│       ├── voicetarget.go       # VoiceTarget type
│       ├── textmessage.go       # TextMessage type
│       └── ban.go               # BanEntry type
├── internal/                    # ── Server Implementation ──
│   ├── server/                  # Virtual server lifecycle
│   ├── transport/               # TCP/TLS and UDP listeners
│   ├── audio/                   # Audio routing and fan-out
│   ├── channel/                 # Channel tree state
│   ├── user/                    # User session lifecycle
│   ├── acl/                     # ACL evaluation engine
│   ├── text/                    # Text message dispatch
│   ├── ban/                     # Ban list management
│   ├── config/                  # Configuration loading
│   ├── database/                # SQLite persistence
│   └── rest/                    # REST API handlers + SPA serving
├── api/
│   └── openapi.yaml             # OpenAPI 3.0 specification
├── configs/
│   └── mumble-server.toml       # Default configuration
├── scripts/                     # Build, packaging, and utility scripts
├── docs/                        # Documentation
├── research/                    # Reference implementations
├── go.mod
└── LICENSE
```

## Documentation

### Core

- [Product Overview](docs/product-overview.md) — Project vision and feature summary
- [Technical Overview](docs/technical-overview.md) — Architecture, subsystems, and design decisions

### Protocol

- [Control Messages](docs/protocol/control-messages.md) — TCP protobuf message catalog (types 0–26)
- [Voice Data](docs/protocol/voice-data.md) — UDP audio packet format and routing
- [Security Modes](docs/protocol/security-modes.md) — Legacy vs secure mode design
- [Encryption](docs/protocol/encryption.md) — TLS, AEAD ciphers, password hashing, storage encryption
- [Permissions](docs/protocol/permissions.md) — Permission bitmask definitions

### Frontend

- [Frontend Guide](docs/patterns/frontend-guide.md) — Vue 3 and Vuetify conventions
- [UI Style Guidelines](docs/patterns/ui-style-guidelines.md) — Visual design standards

### Patterns

- [Connection Lifecycle](docs/patterns/connection-lifecycle-pattern.md) — Client connect through disconnect
- [Handler Table](docs/patterns/handler-table-pattern.md) — Message dispatch by type ID
- [Channel Tree](docs/patterns/channel-tree-pattern.md) — Hierarchical channel management
- [Audio Pipeline](docs/patterns/audio-pipeline-pattern.md) — Voice routing and forwarding
- [ACL Evaluation](docs/patterns/acl-evaluation-pattern.md) — Permission resolution algorithm
- [Concurrent State](docs/patterns/concurrent-state-pattern.md) — Goroutine-safe shared state

## Client Compatibility

go-mumble-server is compatible with any client implementing the standard Mumble protocol:

- [Mumble](https://www.mumble.info/) (Desktop — Windows, macOS, Linux)
- [Plumble](https://play.google.com/store/apps/details?id=com.morlunk.mumbleclient) (Android)
- [Mumla](https://f-droid.org/packages/se.lublin.mumla/) (Android, F-Droid)
- Custom Go clients built on `pkg/mumble/`

## License

MIT License — Copyright (c) 2026 Daniel Chote. See [LICENSE](LICENSE).
