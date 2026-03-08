# Concurrent State Pattern

> **Status:** Design

## Overview

A Mumble server must safely share mutable state (users, channels, ACLs) across multiple goroutines: one per client connection, one for UDP audio, and one for the REST API. This document describes the concurrency strategy for go-mumble-server, replacing the original Murmur's Qt-based threading model with idiomatic Go patterns.

## Murmur's Threading Model (Reference)

Murmur uses two threads per virtual server:

- **Main thread** — Owns all control-plane state (channels, users, ACLs, bans). Runs in the Qt event loop. Handles TCP messages, RPC, and timers.
- **Voice thread** — Handles UDP audio routing. Reads shared state under a read lock.

A `QReadWriteLock` (`qrwlVoiceThread`) synchronizes them: the main thread takes a write lock when modifying state the voice thread reads; the voice thread holds a read lock during audio processing.

## Go Approach

### Goroutine Topology

```
                    ┌────────────────────┐
                    │   Main Server      │
                    │   (startup, meta)  │
                    └─────────┬──────────┘
                              │
          ┌───────────────────┼───────────────────┐
          │                   │                   │
          ▼                   ▼                   ▼
  ┌───────────────┐  ┌───────────────┐   ┌───────────────┐
  │ TCP Listener  │  │ UDP Listener  │   │  REST Server  │
  │ (accept loop) │  │ (read loop)   │   │  (net/http)   │
  └───────┬───────┘  └───────┬───────┘   └───────┬───────┘
          │                   │                   │
          ▼                   │                   │
  ┌───────────────┐           │                   │
  │ Per-Client    │           │                   │
  │ Goroutine ×N  │           │                   │
  └───────┬───────┘           │                   │
          │                   │                   │
          └───────────────────┼───────────────────┘
                              │
                              ▼
                    ┌────────────────────┐
                    │   Shared State     │
                    │   (RWMutex)        │
                    └────────────────────┘
```

### RWMutex for Shared State

The server's shared state (channel tree, user map, ACL cache, ban list) is protected by `sync.RWMutex`:

- **Read lock** — Held by audio routing goroutine (UDP), REST API handlers, and permission checks. These are frequent and must be fast.
- **Write lock** — Held during state mutations: user join/leave, channel create/remove, ACL changes, ban updates. These are infrequent.

```go
type ServerState struct {
    mu       sync.RWMutex
    channels map[uint32]*Channel
    users    map[uint32]*User
    aclCache map[aclCacheKey]Permission
}

func (s *ServerState) GetChannel(id uint32) *Channel {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.channels[id]
}

func (s *ServerState) AddUser(user *User) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.users[user.Session] = user
    s.aclCache = nil // invalidate
}
```

### Per-Client State

State that belongs to a single client (crypto state, write buffer, voice target configuration) is owned exclusively by that client's goroutine and requires no locking.

### Message Passing for Broadcasts

When a state change requires notifying all connected clients (e.g., `UserState` or `ChannelState` broadcast), the server uses a fan-out pattern:

```go
func (s *Server) broadcast(msg []byte) {
    s.state.mu.RLock()
    users := make([]*User, 0, len(s.state.users))
    for _, u := range s.state.users {
        users = append(users, u)
    }
    s.state.mu.RUnlock()

    for _, u := range users {
        u.Send(msg) // non-blocking send to per-client write channel
    }
}
```

Each client has a buffered write channel. Sends are non-blocking — if a client's buffer is full, the message is dropped or the client is disconnected (slow client protection).

### Audio Path Optimization

Audio routing is the hottest path. To minimize lock contention:

1. The UDP goroutine holds a read lock only for the duration of recipient lookup (reading channel membership and links).
2. Actual packet encryption and sendto happen outside the lock.
3. Channel links and user lists are stored in maps that allow concurrent read access under `RLock`.

### Avoiding Deadlocks

Rules to prevent deadlocks:

1. Never hold a write lock while calling into another subsystem that may take a lock.
2. Lock ordering: `ServerState.mu` → `Database.mu` (if needed). Never reverse.
3. Prefer short critical sections — copy data out, then process.
4. REST handlers take a read lock, copy the needed data, release, then serialize to JSON.

## Comparison with Murmur

| Aspect | Murmur | go-mumble-server |
|--------|--------|-----------------|
| Threads | 2 (main + voice) per virtual server | N+3 goroutines (N clients + TCP + UDP + REST) |
| Locking | `QReadWriteLock` | `sync.RWMutex` |
| Event dispatch | Qt event loop + `customEvent()` | Direct function calls under lock |
| Broadcast | Iterate `qhUsers` under lock | Copy user list, fan-out via channels |
| Audio path | Voice thread holds read lock | UDP goroutine holds read lock |

## Reference

- Murmur threading: `research/mumble/docs/dev/MurmurLocking.md`
- gumble locking: `rpwMutex` in `research/gumble/gumble/rpwmutex.go`
