# Channel Tree Pattern

> **Status:** Implemented

## Overview

Mumble organizes voice channels in a hierarchical tree structure. Every server has a single root channel (ID 0) from which all other channels descend. The tree governs navigation, ACL inheritance, and audio routing for linked channels.

## Root Channel ID 0 — DO NOT CHANGE

**The root channel must always have ID 0.** This is required by the Mumble protocol. Mumble clients will not work correctly if the root channel has any other ID.

- **Database**: The root channel row must be stored with `id = 0`. GORM omits zero-value primary keys by default; when creating the root, use `db.Select("ID", ...).Create(&root)` so ID 0 is included in the INSERT.
- **Locations**: `internal/channel/manager.go` (load), `internal/handler/server.go` (GetChannels). Any new code that creates the root channel must use the same Select pattern.
- **Never** auto-increment or assign a different ID to the root. Never add migrations or logic that would change the root's ID.
- **Wire protocol**: `ChannelState` always emits `channel_id` (root = 0); root omits the `parent` field (proto2 optional, so `has_parent()=false` on the client). Non-root channels always include `parent`. `UserState` always emits `session` and `channel_id` (users in root have `channel_id` 0). Messages that reference a channel must emit `channel_id` even when 0.
- **Root ID migration**: The root channel must have ID 0 in the database. If a pre-existing database has the root at a different ID (e.g. due to GORM auto-increment), the channel manager automatically migrates it to 0 on startup, updating all child references, ACLs, and groups.
- **Handlers**: ACL, VoiceTarget, and PermissionQuery treat `channel_id` 0 as the root channel (never map 0 to another channel). Users can be in channel 0; voice targets can include root and its linked channels.

## Data Model

```go
type Channel struct {
    ID          uint32
    Name        string
    Parent      *Channel
    Children    map[uint32]*Channel
    Links       map[uint32]*Channel
    Users       map[uint32]*User
    Listeners   map[uint32]*User  // users listening without joining

    Description string
    Position    int32
    MaxUsers    uint32
    Temporary   bool

    ACLs        []ACLEntry
    Groups      []Group
    InheritACL  bool
}
```

## Tree Operations

### Channel Creation

1. Validate permissions (`MakeChannel` or `MakeTempChannel` on the parent).
2. Assign a unique channel ID.
3. Insert into parent's `Children` map.
4. Broadcast `ChannelState` to all clients.
5. Persist to database (unless temporary).

### Channel Removal

1. Validate permissions (`Write` on the channel).
2. Move all users to the parent channel.
3. Recursively remove child channels.
4. Remove all links referencing this channel.
5. Broadcast `ChannelRemove` to all clients.
6. Delete from database.

### Moving Users

1. Validate permissions (`Enter` on the destination, `Move` if moving others).
2. Remove user from current channel's `Users` map.
3. Add user to destination channel's `Users` map.
4. Broadcast `UserState` with updated channel ID.
5. Clean up empty temporary channels.

### Channel Linking

Linked channels share audio — users in one linked channel hear users in all linked channels.

1. Validate permissions (`LinkChannel` on both channels).
2. Links are bidirectional — if A links to B, B also links to A.
3. Broadcast updated `ChannelState` with link information.

### Channel Listeners

Users can listen to a channel without being present in it:

1. Validate permissions (`Listen` on the target channel).
2. Add user to the channel's `Listeners` map.
3. Audio from the listened channel is forwarded to the listener.
4. Listener counts are limited per channel and per user (configurable).

## Tree Traversal

Several operations require tree traversal:

- **ACL evaluation** — Walk from root to target channel, accumulating ACL entries.
- **Tree-wide text messages** — Broadcast to a channel and all descendants.
- **State synchronization** — Send channels depth-first during client sync.
- **Temporary channel cleanup** — Walk up from an empty temporary channel, removing empty ancestors.

## Constraints

| Constraint | Configuration Key | Default |
|------------|-------------------|---------|
| Max nesting depth | `channelnestinglimit` | 10 |
| Max total channels | `channelcountlimit` | 1000 |
| Channel name regex | `channelname` | (any) |
| Max users per channel | Per-channel `MaxUsers` | 0 (unlimited) |
| Listeners per channel | `listenersperchannel` | — |
| Listeners per user | `listenersperuser` | — |

## Concurrency

The channel tree is read-heavy (audio routing checks links on every voice packet) and write-light (channels change infrequently). An `RWMutex` protects the tree, with audio routing holding a read lock and mutations (create/remove/move) holding a write lock.

## Reference

- Murmur: `Channel` class in `research/mumble/src/Channel.h`
- gumble: `Channel` struct in `research/gumble/gumble/channel.go`
