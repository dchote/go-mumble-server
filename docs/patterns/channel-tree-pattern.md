# Channel Tree Pattern

> **Status:** Implemented

## Overview

Mumble organizes voice channels in a hierarchical tree structure. Every server has a single root channel (ID 0) from which all other channels descend. The tree governs navigation, ACL inheritance, and audio routing for linked channels.

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
