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
| `auth` | Users with a registered account (user ID > 0) |
| `in` | Users currently in this channel |
| `out` | Users not currently in this channel |
| `admin` | Members of the stored `admin` group, or API users with `role=admin` (RBAC) |
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

`InheritACL` is a property of the *child* side of a link: it says whether this channel takes its parent's ACLs. Building the chain for channel C therefore means walking up from C and stopping at the first channel that does not inherit — that channel's own entries still count, its ancestors' do not.

1. Walk up from the target, collecting channels while each one inherits.
2. Evaluate the collected entries outermost-first, so entries closer to the target are applied last and win.
3. The walk always reaches root unless a channel on the way opted out, which is what makes root the place to put server-wide policy.

Storing an entry with `ApplyHere` or `ApplySubs` set to false requires `acl.CreateACL`: both columns default to true in the schema, and GORM omits zero-valued fields with a default from an INSERT, so a plain `Create` silently widens the entry.

## Evaluation Algorithm

To determine permissions for user U in channel C:

```
function evaluateACL(user, channel):
    # Murmur baseline, plus the self-service permissions a registered or admin
    # account always holds. This is the starting point whether or not any ACL
    # exists, so adding the first ACL row cannot strip an admin of their rights.
    granted = defaultPermissions(user)

    # Build the ACL chain, nearest channel last
    chain = []
    current = channel
    while current != nil:
        prepend current.ACLs to chain
        if not current.InheritACL:
            break
        current = current.Parent

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

ACL evaluation is called frequently (every message, every voice packet for permission checks), so results are cached per (subject, channel) pair. A subject is `{SessionID, UserID}` (`acl.Subject`) rather than a bare user ID: every unregistered user has user ID 0, so keying on the account alone would make one guest's channel position and access tokens decide `@in`/`@out` and token-group answers for all of them. The evaluator resolves the live user record by session first and falls back to the user ID for callers that have no session yet, such as picking a landing channel during authentication.

Because `@in`/`@out` membership depends on where the user currently is, and token groups depend on the tokens a session presented, the whole cache is flushed whenever any of these happen:

- A user moves channels, including the forced move when a channel is removed (`Server.invalidateACLCache`)
- A client authenticates, bringing a channel and a set of access tokens
- ACLs or groups are modified through the REST API (`onACLChange`)

The original Murmur uses `ChanACL::ACLCache` (a per-server `QHash`) that is cleared on any ACL-affecting change.

## Default ACLs

The root channel (ID 0) has a default ACL matching Murmur's baseline:

| Group | Permissions |
|-------|------------|
| `all` | Grant: `Traverse`, `Enter`, `Speak`, `Whisper`, `TextMessage`, `Listen` |
| `auth` | Grant: `MakeTempChannel`, `SelfRegister` |
| `admin` | Grant: `Write` (implies all) |

All users start with the baseline permissions above; ACL entries add or deny from this set. The `auth` group applies to registered users (userID > 0); `admin` applies to stored admin group members or API users with `role=admin`.

## RBAC Strategy (API Users)

Management API users (`users` table) can authenticate to Mumble with their web credentials. For RBAC compliance, they receive synthetic Mumble userIDs in a reserved range so the evaluator can resolve roles:

| User Type | userID | @admin | @auth |
|-----------|--------|--------|-------|
| Unregistered (guest / server password) | 0 | ✗ | ✗ |
| Registered user (per-server) | 1, 2, 3… | If in stored admin group | ✓ |
| API admin (role=admin) | 0x80000000 \| users.id | ✓ | ✓ |
| API user (role=user) | 0x80000000 \| users.id | ✗ | ✓ |

- **Constants**: `APIUserIDBase = 0x80000000`, `APIUserIDMask = 0x7FFFFFFF`
- **Resolution**: When evaluating `@admin`, if `userID >= APIUserIDBase`, the evaluator queries `users` for that id and returns true if `role = 'admin'`
- **Implementation**: `internal/acl/rbac_strategy.go` — `MakeAPIUserID`, `IsAPIUserID`, `ResolveAPIAdmin`
- **Auth**: `handleAuthenticate` assigns synthetic userIDs to API users; `LookupAPIUser` returns id and role
- **Display**: REST `GET /api/v1/servers/:id/users` returns `is_admin` per connected user for channel tree UI
- **Note**: API users must have `role=admin` in the `users` table for `is_admin` to be true. Use the admin users page (`/admin/users`) to set or update roles.

## Implementation

- **Evaluator**: `internal/acl/evaluator.go` — `NewEvaluator(db, chans, users)`, `Check()`, `EffectivePermissions()`, `InvalidateCache()`; callers build subjects with `SubjectOf(user)` or `SubjectForUserID(id)`
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
