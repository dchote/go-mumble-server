# 0008: UserState Authorization, Content Limits and ACL Chain Fixes

## Status: Implemented

Follows [0007](0007-userstate-field-presence.md), which fixed the *shape* of
`UserState` messages. This one fixes who is allowed to send them, what they may
carry, and the permission machinery that decides.

## Summary

Reviewing `handleUserState` against `Server::msgUserState` in
`research/mumble/src/murmur/Messages.cpp` turned up a set of authorization checks
Murmur performs that this server did not, and the work of adding them exposed
three deeper faults in the permission layer: the root channel was not where
`RootID()` said it was, the ancestor walk never reached root, and ACL rows could
not store a false `apply_here` / `apply_subs`. Together those meant that on a
fresh server **no root ACL had any effect anywhere**, which is also why the
missing checks had gone unnoticed.

## Authorization added to `handleUserState`

| Rule | Murmur reference | Behaviour when violated |
| --- | --- | --- |
| SuperUser may only be modified by SuperUser, ahead of every other check | Messages.cpp:775-779 | `PermissionDenied{SuperUser}` |
| Administrative `mute`/`deaf`/`suppress` on a target inside a temporary channel re-check `MuteDeafen` in the first permanent ancestor | Messages.cpp:871-887 | `PermissionDenied{TemporaryChannel}` |
| Another user's `comment`/`texture` requires `ResetUserContent` on root, and may only *clear* it | Messages.cpp:890-929 | `PermissionDenied{Permission}` / `{TextTooLong}` |
| `comment` and `texture` are bounded by the configured limits | Messages.cpp:906-919 | `PermissionDenied{TextTooLong}` |
| A channel move needs `Move` on the source when moving somebody else, and either `Move` on the destination or `Enter` for the target | Messages.cpp:800-813 | `PermissionDenied{Permission}` |
| A channel that has reached `MaxUsers` cannot be entered (a `Write` holder is exempt) | Messages.cpp:814-817, `Server::isChannelFull` | `PermissionDenied{ChannelFull}` |
| Starting a recording on a server with recording disabled disconnects the client | Messages.cpp:1048-1067 | `UserRemove` with a reason, then disconnect |
| Self-targeted `UserState` is rate limited to 10/second | Messages.cpp:790 | dropped silently, as upstream |

`joinChannelFor` applies the same capacity rule at connect time and falls back to
root, so a full default channel cannot wedge a login.

**Identifying SuperUser.** Murmur reserves registered user ID 0 for SuperUser and
uses -1 for unregistered users. Here 0 already means "unregistered", so SuperUser
is identified by the reserved name (`mumble.SuperUserName`) and the flag is only
set for a connection that authenticated against a real account — an anonymous
client claiming the name is rejected with `RejectWrongUserPW`, otherwise the
immunity would be free to take.

## Permission layer fixes

### The root channel was not at ID 0

`RootID()` returns 0 because the protocol fixes it there, but GORM leaves a
zero-valued primary key out of an INSERT — naming it in `Select` does not change
that — so a brand new server got a root at ID 1. Every root-scoped check
(`Ban`, `Kick`, `Register`, `ResetUserContent`, the seeded default ACLs) therefore
resolved against a channel that did not exist and silently fell back to defaults.
`channel.Manager.load` now runs the existing `fixRootID` migration on the row it
just created, so the root is at 0 from the first start. The REST `GetChannels`
endpoint no longer seeds a root of its own; it goes through the manager.

### The ancestor walk stopped short of root

Root reports `parent_id = 0`, and so does every direct child of root. The walk
terminated on that parent value, so a channel's chain never included root and
root's ACLs never reached anything below it. It now terminates on the node
(`id == 0`) and guards against cycles.

### ACL entries could not be scoped

`apply_here` and `apply_subs` default to true in the schema, and GORM omits any
zero-valued field that has a default, so `false` was silently stored as `true` —
every entry written through the REST ACL editor applied everywhere. Inserts now go
through `acl.CreateACL` / `acl.CreateGroup`, which write an explicit column map.
The same trap applied to a group's `inherit` / `inheritable`.

### `InheritACL` was read from the wrong side of the link

`InheritACL` says whether a channel takes *its parent's* ACLs. The evaluator was
testing the ancestor's own flag instead, so a channel that opted out still
inherited. The chain now stops at the first channel that does not inherit, whose
own entries still count (`ACL.cpp`, `hasPermission`).

### Baseline permissions were dropped as soon as any ACL existed

`evaluate` started from a hard-coded copy of the Murmur baseline while the
no-ACL path started from `defaultRootPerms`, which additionally grants
`MakeTempChannel` / `SelfRegister` to registered accounts and `Write` to API
admins. Adding the first ACL row therefore stripped an API admin of `Write`. Both
paths now start from `defaultRootPerms`, and the baseline mask itself lives in one
place, `acl.DefaultPermissions`.

### ACL cache staleness

Group membership depends on where a user is (`@in` / `@out`) and on the tokens the
session presented. The cache was only flushed on a REST ACL edit and on a
`UserState` channel move; it is now also flushed when a client authenticates and
when users are force-moved by a channel removal, all through
`Server.invalidateACLCache`.

The evaluator and its cache also keyed on the registered user ID, and every
unregistered user has ID 0, so one guest's channel and access tokens decided
`@in` / `@out` and token-group answers for every other guest. Permissions are now
resolved for an `acl.Subject` — `{SessionID, UserID}` — built with
`acl.SubjectOf(user)`, or `acl.SubjectForUserID(id)` for the one caller that runs
before a session exists (`joinChannelFor`). The evaluator looks the user up by
session first and falls back to the account.

## Concurrency

`user.Manager` used to hand out live `*mumble.User` pointers (`GetUser`,
`GetByName`, `ListAll`, `ListByChannel`) that callers read after the lock was
released, racing the control goroutine's writes. Those methods are gone. Readers
use `Snapshot`, `SnapshotByName`, `SnapshotByUserID`, `SnapshotAll` or
`SnapshotByChannel`, all of which return deep copies; callers that only need a
fact use `Exists`, `CountInChannel`, `ChannelID` or `SessionIDsInChannel`. `Add`
and `Remove` take and return values. `userToState` now takes a `mumble.User` by
value so it cannot alias a live texture slice.

`channel.Manager` had the same defect and now has the same fix: `GetChannel`,
`GetChannelWithMeta`, `GetTree` and `Create` return copies, because `Update`
rewrites the stored channels in place while the sync loop is serialising them.

Every ACL question is asked through `Server.aclCheck` / `aclPermissions`, which
answer with the baseline when no evaluator is configured, so nil handling is
identical everywhere and matches `maySpeak`'s lock-ordering rule from 0007.

## Kick and ban delivery

Outbound messages are queued to a per-connection channel and written by a separate
goroutine, so closing the socket immediately after queueing a `UserRemove`
discarded the very message that explains the disconnect. Kick, ban and the
recording disconnect now use `Conn.CloseAfterFlush`, which waits up to 250 ms for
the queue to drain — Murmur's `forceFlush()` before `disconnectSocket()`.
`Conn.Close` also no longer closes the write channel; it closes a `done` channel
that the writer selects on, so a broadcast racing a disconnect returns
`net.ErrClosed` instead of panicking on a send to a closed channel.

## Configuration

Three per-server settings, defaulting to Murmur's values:

| Setting | Default | Effect |
| --- | --- | --- |
| `allow_recording` | true | When false, a client that starts recording is disconnected |
| `max_text_message_length` | 5000 | Bounds chat messages and user comments; 0 = unlimited |
| `max_image_message_length` | 131072 | Bounds image messages and user textures; 0 = unlimited |

They are stored in `server_configs`, exposed through
`GET`/`PATCH /api/v1/servers/{id}/config`, editable on the server config page, and
applied without a restart via `Server.SetContentPolicy` from the existing
`onConfigChange` hook. GORM's `AutoMigrate` backfills existing databases with the
column defaults.

## Files modified

- `internal/mumble/handlers.go` — authorization checks, content policy atomics, `invalidateACLCache`, `aclCheck`/`aclPermissions`, snapshot migration, shared `banEntry`/`banEntryByIP`/`banEntryByCert` helpers
- `internal/user/manager.go` — snapshot API, value-based `Add`/`Remove`
- `internal/acl/evaluator.go` — inheritance stop rule, baseline unification, `DefaultPermissions`, `Subject`-keyed evaluation and cache
- `internal/acl/seed.go` — `CreateACL` / `CreateGroup`
- `internal/channel/manager.go` — root at ID 0, `AncestorChain` reaches root, accessors return copies, `GetTree` no longer guesses a root
- `internal/connection/conn.go` — `CloseAfterFlush`, `done` channel, `SetUser` reduced to `SetUserName` (the stored user ID and channel had no readers)
- `internal/handler/server.go`, `internal/handler/acl.go` — config fields, root seeding via the manager, ACL inserts
- `internal/config/config.go`, `internal/config/dbconfig.go`, `internal/database/models/server_config.go` — new settings
- `internal/database/models/channel.go` — corrected root ID comment
- `internal/server/server.go` — content policy on config change
- `pkg/mumble/user.go` — `IsSuperUser`, `SuperUserName`
- `api/openapi.yaml`, `frontend/src/pages/servers/[id]/config.vue`
- `Makefile`, `.github/workflows/ci.yml` — `gofmt` gate and `make check`; the file list comes from a tree walk instead of `git ls-files`, which skipped new files and choked on deleted ones; `vet`/`test` use `./cmd/... ./internal/... ./pkg/...` so frontend `node_modules` Go sources are never walked; CI runs `make fmt-check` / `make vet` / `make test` so both gates stay identical

Removed as unreferenced: `pkg/mumble/{acl,version,textmessage,ban,voicetarget}.go`,
`ocb2.NewBlock`, `audio.NewRouter` (superseded by `NewRouterWithConfig`),
`user.Manager.RegisterDBUser` (a thin wrapper over `LookupAPIUser`),
`channel.Manager.NextID` and `Conn.UserChannel`. The deleted `pkg/mumble` types
described protocol data that `pkg/mumble/protocol/messages` already carries on the
wire, so nothing outside them could have been built against them.

## Tests

- `internal/mumble/handlers_userstate_permissions_test.go` — SuperUser immunity (and that SuperUser can still change itself), cross-user comment/texture, size limits, channel-move ACLs, channel full, temporary-channel escalation, recording disconnect and the always-allowed stop, `UserState` rate limiting, full-default-channel fallback.
- `internal/channel/manager_test.go` — root created at 0, a legacy root migrated to 0 with its children, `AncestorChain` reaching root, `GetTree` announcing parents before children, accessors returning copies.
- `internal/mumble/handlers_channelstate_test.go` — root's `ChannelState` omitting `parent` on the struct and on the wire (field 2 absent after Marshal); children carry parent field 2. A root that claims `parent = 0` crashes Mumla mid-sync.
- `internal/acl/evaluator_test.go` — root ACLs inheriting into subchannels, `ApplySubs=false` staying put, a non-inheriting channel ignoring ancestors while keeping its own, the baseline surviving the first ACL row, two anonymous sessions getting independent `@in`/`@out` answers, unknown channels falling back to the baseline.
- Verified with `make check` (`gofmt`, `go vet`, `go test -race ./...`).
