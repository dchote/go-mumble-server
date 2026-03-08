# Permissions

> **Status:** Reference — derived from `research/mumble/src/ACL.h` and `research/gumble/gumble/permission.go`

## Overview

Mumble uses a bitmask-based permission system. Each permission is a single bit in a 32-bit integer. ACL entries grant or deny combinations of these bits. The effective permission for a user in a channel is the result of evaluating the ACL chain from root to that channel.

See [patterns/acl-evaluation-pattern.md](../patterns/acl-evaluation-pattern.md) for the evaluation algorithm.

## Permission Bits

| Bit | Value | Name | Description |
|-----|-------|------|-------------|
| 0 | 0x00000001 | Write | Full control. Implies all other permissions. |
| 1 | 0x00000002 | Traverse | Can traverse (pass through) the channel to reach sub-channels. |
| 2 | 0x00000004 | Enter | Can enter the channel. |
| 3 | 0x00000008 | Speak | Can transmit voice in the channel. |
| 4 | 0x00000010 | MuteDeafen | Can mute or deafen other users. |
| 5 | 0x00000020 | Move | Can move users between channels. |
| 6 | 0x00000040 | MakeChannel | Can create permanent sub-channels. |
| 7 | 0x00000080 | LinkChannel | Can link this channel to other channels. |
| 8 | 0x00000100 | Whisper | Can whisper to this channel (directed audio). |
| 9 | 0x00000200 | TextMessage | Can send text messages to this channel. |
| 10 | 0x00000400 | MakeTempChannel | Can create temporary sub-channels. |
| 11 | 0x00000800 | Listen | Can listen to this channel without joining. |

### Root-Only Permissions

These permissions are only evaluated on the root channel (ID 0):

| Bit | Value | Name | Description |
|-----|-------|------|-------------|
| 16 | 0x00010000 | Kick | Can kick users from the server. |
| 17 | 0x00020000 | Ban | Can ban users from the server. |
| 18 | 0x00040000 | Register | Can register other users. |
| 19 | 0x00080000 | SelfRegister | Can register themselves. |
| 20 | 0x00100000 | ResetUserContent | Can reset user comments and avatars. |

## Special Permission: Write

The `Write` permission (bit 0) is the superuser permission. A user with `Write` on a channel effectively has all permissions on that channel. When checking permissions, if `Write` is granted, all other bits are implicitly set.

## Permission Checks

Common operations and their required permissions:

| Operation | Required Permission | Channel |
|-----------|---------------------|---------|
| Enter a channel | `Enter` + `Traverse` on path | Target channel |
| Speak in channel | `Speak` | Current channel |
| Send text message | `TextMessage` | Target channel |
| Move self to channel | `Enter` on target | Target channel |
| Move another user | `Move` on source or target | Source/target |
| Mute/deafen a user | `MuteDeafen` | User's channel |
| Create permanent channel | `MakeChannel` | Parent channel |
| Create temporary channel | `MakeTempChannel` | Parent channel |
| Remove channel | `Write` | Target channel |
| Link channels | `LinkChannel` | Both channels |
| Whisper to channel | `Whisper` | Target channel |
| Listen to channel | `Listen` | Target channel |
| Kick a user | `Kick` | Root channel |
| Ban a user | `Ban` | Root channel |
| Register a user | `Register` | Root channel |
| Register self | `SelfRegister` | Root channel |

## Go Type Definition

```go
type Permission uint32

const (
    PermissionWrite           Permission = 0x00000001
    PermissionTraverse        Permission = 0x00000002
    PermissionEnter           Permission = 0x00000004
    PermissionSpeak           Permission = 0x00000008
    PermissionMuteDeafen      Permission = 0x00000010
    PermissionMove            Permission = 0x00000020
    PermissionMakeChannel     Permission = 0x00000040
    PermissionLinkChannel     Permission = 0x00000080
    PermissionWhisper         Permission = 0x00000100
    PermissionTextMessage     Permission = 0x00000200
    PermissionMakeTempChannel Permission = 0x00000400
    PermissionListen          Permission = 0x00000800

    // Root-only
    PermissionKick             Permission = 0x00010000
    PermissionBan              Permission = 0x00020000
    PermissionRegister         Permission = 0x00040000
    PermissionSelfRegister     Permission = 0x00080000
    PermissionResetUserContent Permission = 0x00100000
)

func (p Permission) Has(check Permission) bool {
    if p&PermissionWrite != 0 {
        return true
    }
    return p&check == check
}
```

## Default Root Channel ACLs

The root channel ships with these default ACL entries:

| Priority | Group | Apply Here | Apply Subs | Grant | Deny |
|----------|-------|------------|------------|-------|------|
| 1 | `all` | yes | yes | `Traverse`, `Enter` | — |
| 2 | `auth` | yes | yes | `Speak`, `TextMessage`, `MakeTempChannel`, `SelfRegister` | — |
| 3 | `admin` | yes | yes | `Write` | — |

## SuperUser

User ID 0 is the SuperUser account:

- Always a member of the `admin` group on every channel.
- Always has `Write` permission regardless of ACLs.
- Cannot be kicked, banned, or have permissions denied.
- Does not count against user limits.

## Reference

- Permission definitions: `research/mumble/src/ACL.h` (enum `Perm`)
- gumble permissions: `research/gumble/gumble/permission.go`
- ACL evaluation: `research/mumble/src/ACL.cpp` (`ChanACL::hasPermission`)
- Default ACLs: `research/mumble/src/murmur/ServerDB.cpp` (table creation)
