# ACL Evaluation Pattern

> **Status:** Implemented

## Overview

Mumble uses a hierarchical Access Control List (ACL) system to govern what users can do in each channel. ACLs are defined per-channel and inherit down the channel tree. Groups provide named collections of users that ACL entries reference.

## Core Concepts

### Groups

A group is a named set of users defined on a channel. Groups support inheritance — a group defined on a parent channel is visible to child channels unless inheritance is disabled.

| Property | Description |
|----------|-------------|
| `Name` | Group identifier |
| `Inherit` | Whether this channel inherits members from the parent's group of the same name |
| `Inheritable` | Whether child channels can inherit this group |
| `Add` | User IDs explicitly added to the group on this channel |
| `Remove` | User IDs explicitly removed from the group on this channel |

**Built-in groups** (computed, not stored):

| Group | Members |
|-------|---------|
| `all` | Every connected user |
| `auth` | Users with a registered account (user ID ≥ 0) |
| `in` | Users currently in this channel |
| `out` | Users not currently in this channel |
| `admin` | Members of the `admin` group |
| `sub` | Users in sub-channels (with optional path/depth constraints) |
| `~channel` | Evaluated in the context of the channel being ACL-checked (not the user's current channel) |

**Token groups:** A group name prefixed with `#` is a token group. Users who supply a matching access token in their `Authenticate` message are considered members.

### ACL Entries

Each ACL entry maps a user or group to a set of granted and denied permissions:

| Property | Description |
|----------|-------------|
| `ApplyHere` | Entry applies to this channel |
| `ApplySubs` | Entry applies to sub-channels |
| `UserID` | Specific user (mutually exclusive with Group) |
| `Group` | Group name (or meta: all, auth, in, out, admin, sub) |
| `AccessToken` | Token for @# token groups |
| `EvalHere` | ~ prefix: resolve @in/@out/@sub in ACL-definition channel context |
| `Invert` | ! prefix: invert the selector match |
| `Grant` | Permission bits to allow |
| `Deny` | Permission bits to deny |

### Inheritance

Channels inherit ACLs from their parent unless `InheritACL` is set to false. When inheriting:

1. Start with the parent channel's effective ACL list.
2. Append this channel's own ACL entries.
3. Evaluate in order — later entries override earlier ones.

## Evaluation Algorithm

To determine permissions for user U in channel C:

```
function evaluateACL(user, channel):
    # Build the ACL chain from root to this channel
    chain = []
    current = channel
    while current != nil:
        if current.InheritACL or current == channel:
            prepend current.ACLs to chain
        if not current.InheritACL:
            break
        current = current.Parent

    granted = 0
    denied = 0

    for each entry in chain:
        # Check applicability
        if entry is on the target channel and not entry.ApplyHere:
            continue
        if entry is on an ancestor channel and not entry.ApplySubs:
            continue

        # Check if the entry matches this user
        if entry.UserID is set:
            if entry.UserID != user.UserID:
                continue
        else:
            if user not in resolveGroup(entry.Group, channel):
                continue

        # Apply grants and denials
        granted |= entry.Grant
        granted &= ~entry.Deny
        denied |= entry.Deny
        denied &= ~entry.Grant

    # SuperUser always has Write permission
    if user is SuperUser:
        granted |= Write

    return granted & ~denied
```

## Group Resolution

Resolving group membership for a user in a channel context:

```
function resolveGroup(groupName, channel):
    if groupName is a built-in group:
        return computeBuiltinGroup(groupName, channel)

    if groupName starts with '#':
        return users whose access tokens contain groupName[1:]

    # Walk up the tree collecting members
    members = {}
    current = channel
    while current != nil:
        group = current.Groups[groupName]
        if group exists:
            if group.Inherit or current == channel:
                members = members ∪ group.Add
                members = members \ group.Remove
            if not group.Inheritable:
                break
        current = current.Parent

    return members
```

## Permission Bits

See [protocol/permissions.md](../protocol/permissions.md) for the full bitmask definition.

## Caching

ACL evaluation is called frequently (every message, every voice packet for permission checks). Results should be cached per (user, channel) pair and invalidated when:

- A user moves channels
- ACLs or groups are modified on any channel
- A user connects or disconnects
- Access tokens change

The original Murmur uses `ChanACL::ACLCache` (a per-server `QHash`) that is cleared on any ACL-affecting change.

## Default ACLs

The root channel (ID 0) has a default ACL:

| Group | Permissions |
|-------|------------|
| `admin` | Grant: `Write` (implies all) |
| `auth` | Grant: `Speak`, `TextMessage`, `MakeTempChannel`, `SelfRegister` |
| `all` | Grant: `Traverse`, `Enter` |

SuperUser (user ID 0) is always in the `admin` group and always has `Write` permission.

## Implementation

- **Evaluator**: `internal/acl/evaluator.go` — `NewEvaluator(db, chans, users)`, `Check()`, `EffectivePermissions()`, `InvalidateCache()`
- **Seed**: `internal/acl/seed.go` — `EnsureDefaultRootACLs()` seeds default groups and ACLs for root channel
- **Models**: `internal/database/models/channel_acl.go`, `channel_group.go`
- **User**: `pkg/mumble/user.go` — `AccessTokens` for token group membership
- **Channel**: `pkg/mumble/channel.go` — `InheritACL` for inheritance control
- **Cache invalidation**: On ACL PUT (via REST), on user channel move; REST handler receives `OnACLChange` callback

## Reference

- Murmur ACL evaluation: `ChanACL::hasPermission()` in `research/mumble/src/ACL.cpp`
- Murmur groups: `Group` class in `research/mumble/src/Group.h`
- gumble ACL: `research/gumble/gumble/acl.go`
- Mumble ACL guide: https://www.mumble.info/documentation/administration/acl/
