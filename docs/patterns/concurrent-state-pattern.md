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

Shared state is split across one manager per concern — `channel.Manager`, `user.Manager`, the ACL evaluator's cache, `ban.Manager` — each with its own `sync.RWMutex`:

- **Read lock** — Held by audio routing goroutine (UDP), REST API handlers, and permission checks. These are frequent and must be fast.
- **Write lock** — Held during state mutations: user join/leave, channel create/remove, ACL changes, ban updates. These are infrequent.

No manager hands out a pointer into its own state. Readers get a deep copy, and writers pass a mutation function that runs under the lock:

```go
func (m *Manager) Snapshot(sessionID uint32) (mumble.User, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    u, ok := m.bySession[sessionID]
    if !ok {
        return mumble.User{}, false
    }
    return u.Clone(), true // texture and tokens copied too
}

func (m *Manager) UpdateUser(sessionID uint32, fn func(*mumble.User)) (mumble.User, bool)
```

`channel.Manager` follows the same rule: `GetChannel`, `GetChannelWithMeta`, `GetTree` and `Create` return `*mumble.Channel` values that point at private copies, links slice included. `Update` rewrites the stored channels in place under the write lock, so a caller holding a stored pointer — the sync loop serialising the tree, for instance — would otherwise read a channel mid-rewrite.

**Lock ordering:** a mutation function passed to `UpdateUser` runs while the user manager's lock is held, and the ACL evaluator reads user records to resolve group membership. Resolve every permission question *before* calling `UpdateUser` and pass the answer in — calling the evaluator from inside the closure deadlocks.

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
4. Prefer the narrow accessors — `user.Manager.SpeakGateFor`, `VoiceState`, `ChannelID`, `SessionIDsInChannel` — on the voice path. They answer one question under the lock instead of copying a whole record. `Snapshot`, `SnapshotAll`, and `SnapshotByChannel` return **deep** copies (including texture and tokens) and belong on the control path.

### Avoiding Deadlocks

Rules to prevent deadlocks:

1. Never hold a write lock while calling into another subsystem that may take a lock. In particular, never call the ACL evaluator from inside a `UpdateUser` closure.
2. Lock ordering: manager mutex → `Database.mu` (if needed). Never reverse.
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
