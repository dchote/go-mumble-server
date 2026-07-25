# Channel Tree Pattern

> **Status:** Implemented

## Overview

Mumble organizes voice channels in a hierarchical tree structure. Every server has a single root channel (ID 0) from which all other channels descend. The tree governs navigation, ACL inheritance, and audio routing for linked channels.

## Root Channel ID 0 — DO NOT CHANGE

**The root channel must always have ID 0.** This is required by the Mumble protocol. Mumble clients will not work correctly if the root channel has any other ID.

- **Database**: The root channel row must be stored with `id = 0`. GORM leaves a zero-valued primary key out of the INSERT — naming it in `Select` does *not* change that — so a freshly created root is auto-assigned an ID and then migrated back to 0 (see *Root ID migration* below).
- **Locations**: `channel.Manager.load()` in `internal/channel/manager.go` is the only place that creates a root channel. Everything else, including the REST `GetChannels` endpoint, seeds through the manager so the migration always runs.
- **Never** assign a different ID to the root, and never add migrations or logic that would change it afterwards.
- **Wire protocol**: `ChannelState` always emits `channel_id` (root = 0); root omits the `parent` field (proto2 optional, so `has_parent()=false` on the client). Non-root channels always include `parent`. `UserState` **snapshots** (join/roster sync) always emit `session` and `channel_id` (users in root have `channel_id` 0); **delta echoes** omit `channel_id` unless the client requested a move — see [0007](../features/0007-userstate-field-presence.md). Messages that reference a channel must emit `channel_id` even when 0.
- **Root ID migration**: `fixRootID` runs on every load. Whenever the row with no parent is not at ID 0, it is moved there along with all child references, ACLs, and groups. This covers both a database written by an older version and the auto-assigned ID of a brand new one.
- **Ancestor walks**: root reports `parent_id = 0`, and so does every direct child of root, so a walk up the tree must terminate on the *node* (`id == 0`) and not on the parent value. Terminating on the parent value drops root from the chain, which silently disables root's ACLs for every other channel.
- **Handlers**: ACL, VoiceTarget, and PermissionQuery treat `channel_id` 0 as the root channel (never map 0 to another channel). Users can be in channel 0; voice targets can include root and its linked channels.

## Data Model

`pkg/mumble.Channel` carries only the channel's own attributes; the tree structure lives in `channel.Manager` (`internal/channel/manager.go`), which wraps each channel in a node holding its children.

```go
type Channel struct {
    ID          uint32
    ParentID    uint32
    Name        string
    Description string
    Position    int32
    MaxUsers    uint32
    IsTemporary bool
    Links       []uint32
    InheritACL  bool // false = do not take the parent's ACLs
}
```

Membership is deliberately not part of the channel: users belong to `user.Manager` and are found by channel through `SnapshotByChannel` / `SessionIDsInChannel`, and ACL entries and groups are read from the database by the ACL evaluator. Keeping one owner per piece of state is what lets the audio path take a read lock on the tree without touching user state.

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
2. Update the user's channel in `user.Manager`.
3. Invalidate the ACL cache — `@in`/`@out` group membership depends on where the user is.
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

- **ACL evaluation** — `AncestorChain` returns the chain target-first, root-last; the evaluator then accumulates entries in the opposite order so the closest channel wins.
- **Tree-wide text messages** — Broadcast to a channel and all descendants.
- **State synchronization** — Send channels depth-first during client sync. `GetTree` walks from root so a parent is always announced before its children, which clients depend on: Mumla's protocol library resolves a `ChannelState`'s parent as it arrives and dereferences it without a null check (`humla ModelHandler.messageChannelState`), so an out-of-order announcement, or a root that claims `parent = 0`, drops the connection mid-sync.
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

The channel tree is read-heavy (audio routing checks links on every voice packet) and write-light (channels change infrequently). An `RWMutex` protects the tree, with audio routing holding a read lock and mutations (create/remove/move) holding a write lock. `Update` rewrites stored channels in place under that lock, so every accessor returns a copy rather than a pointer into the tree — see [concurrent-state-pattern.md](concurrent-state-pattern.md).

## Reference

- Murmur: `Channel` class in `research/mumble/src/Channel.h`
- gumble: `Channel` struct in `research/gumble/gumble/channel.go`
