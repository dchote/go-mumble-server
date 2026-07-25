# Control Messages (TCP)

> **Status:** Reference — field layout matches upstream Mumble spec; we use native Go structs, not protobuf

## Overview

The Mumble control channel uses TCP with TLS. Messages use a Mumble-compatible wire format (field-tagged, length-delimited) and are framed with a 6-byte header. go-mumble-server implements this with native Go structs and hand-written wire encoding — no protobuf library.

## Framing

```
┌──────────────────┬──────────────────────────┬─────────────────────┐
│  Type (uint16)   │  Payload Length (uint32)  │  Message Payload    │
│  2 bytes, BE     │  4 bytes, BE             │  variable length    │
└──────────────────┴──────────────────────────┴─────────────────────┘
```

All multi-byte integers are big-endian. Maximum payload length should be enforced to prevent memory exhaustion (Murmur uses 8 MiB as a practical upper bound for blob messages).

## Message Catalog

### Type 0 — Version

Exchanged immediately after TLS handshake. Both client and server send this.

| Field | Type | Description |
|-------|------|-------------|
| `version_v1` | uint32 | Legacy version (major×65536 + minor×256 + patch) |
| `version_v2` | uint64 | Semantic version (major×65536² + minor×65536 + patch) |
| `release` | string | Human-readable release string |
| `os` | string | Operating system |
| `os_version` | string | OS version string |

### Type 1 — UDPTunnel

Binary audio data tunneled over TCP when UDP is unavailable. The payload is **not** protobuf-encoded — it is the raw audio packet (same format as a decrypted UDP packet).

See [voice-data.md](voice-data.md) for the audio packet format.

### Type 2 — Authenticate

Sent by the client after receiving the server's `Version` and `CryptSetup`.

| Field | Type | Description |
|-------|------|-------------|
| `username` | string | Requested username |
| `password` | string | Server password or registered user password |
| `tokens` | string[] | Access tokens for ACL group membership |
| `celt_versions` | int32[] | Supported CELT codec versions |
| `opus` | bool | Whether the client supports Opus |
| `client_type` | int32 | 0 = regular, 1 = bot |

### Type 3 — Ping

Bidirectional keepalive. Sent periodically by both client and server.

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | uint64 | Originator's timestamp (echoed back) |
| `good` | uint32 | Good (on-time) packets |
| `late` | uint32 | Late packets |
| `lost` | uint32 | Lost packets |
| `resync` | uint32 | Nonce resynchronizations |
| `udp_packets` | uint32 | UDP packets sent |
| `tcp_packets` | uint32 | TCP packets sent |
| `udp_ping_avg` | float | Average UDP ping (ms) |
| `udp_ping_var` | float | UDP ping variance |
| `tcp_ping_avg` | float | Average TCP ping (ms) |
| `tcp_ping_var` | float | TCP ping variance |

### Type 4 — Reject

Sent by the server when authentication fails.

| Field | Type | Description |
|-------|------|-------------|
| `type` | RejectType | Rejection reason (see below) |
| `reason` | string | Human-readable reason |

**RejectType enum:**

| Value | Name | Description |
|-------|------|-------------|
| 0 | None | |
| 1 | WrongVersion | Client version too old/new |
| 2 | InvalidUsername | Username is invalid |
| 3 | WrongUserPW | Incorrect user password |
| 4 | WrongServerPW | Incorrect server password |
| 5 | UsernameInUse | Username already connected |
| 6 | ServerFull | Max users reached |
| 7 | NoCertificate | Client certificate required |
| 8 | AuthenticatorFail | External authenticator failed |

### Type 5 — ServerSync

Sent by the server to complete the authentication/sync phase.

| Field | Type | Description |
|-------|------|-------------|
| `session` | uint32 | Client's assigned session ID |
| `max_bandwidth` | uint32 | Maximum bandwidth per user (bps) |
| `welcome_text` | string | Server welcome message (HTML) |
| `permissions` | uint64 | Client's permissions in the root channel |

### Type 6 — ChannelRemove

Sent by the server when a channel is deleted, or by a client requesting deletion.

| Field | Type | Description |
|-------|------|-------------|
| `channel_id` | uint32 | Channel to remove |

### Type 7 — ChannelState

Full or partial channel state. Sent during sync (full state) and on updates (partial).

**Wire format:** `channel_id` is always sent (root = 0). Root omits the `parent` field entirely (proto2 optional — `has_parent()=false` on the client). Non-root channels always include `parent`.

| Field | Type | Description |
|-------|------|-------------|
| `channel_id` | uint32 | Channel ID (always sent; root = 0) |
| `parent` | uint32 | Parent channel ID (omitted for root) |
| `name` | string | Channel name |
| `links` | uint32[] | Linked channel IDs |
| `description` | string | Channel description (HTML) |
| `links_add` | uint32[] | Links to add (incremental) |
| `links_remove` | uint32[] | Links to remove (incremental) |
| `temporary` | bool | Temporary channel flag |
| `position` | int32 | Sort position |
| `description_hash` | bytes | SHA-1 hash of description |
| `max_users` | uint32 | Max users (0 = unlimited) |
| `is_enter_restricted` | bool | Whether Enter permission is required |
| `can_enter` | bool | Whether the receiving user can enter |

### Type 8 — UserRemove

User disconnected or was kicked/banned.

| Field | Type | Description |
|-------|------|-------------|
| `session` | uint32 | Session of the removed user |
| `actor` | uint32 | Session of the user who kicked (0 = server) |
| `reason` | string | Kick/ban reason |
| `ban` | bool | Whether this is a ban (not just a kick) |

### Type 9 — UserState

Full or partial user state. `Mumble.proto` is **proto2**: every field has explicit
presence — a field that was *assigned* (even to its zero value) is written to the
wire, and "absent" is a distinct state from "false". But presence is not the same
thing as "always send every field": Murmur only ever sends the fields that are
actually relevant to a given message, and clients treat a field's mere presence in
`UserState` as a discrete change notification, not as a full state refresh. See
[protocol-encoding.md](../architecture/protocol-encoding.md) and
[0007-userstate-field-presence.md](../features/0007-userstate-field-presence.md)
for the full rationale and the regression that motivated it.

This server sends `UserState` in two distinct shapes, matching Murmur:

- **Snapshot** (`userToState`, sent once per user during login sync / when a new
  user joins): only *true* voice flags are included. `deaf`/`mute` and
  `self_deaf`/`self_mute` are mutually exclusive on the wire — e.g. a deafened
  user's snapshot carries `deaf=true` but omits `mute`, since deaf already implies
  mute. `session` and `channel_id` are always present (root = 0 is a real value,
  not absence).
- **Delta echo** (`handleUserState`, sent in response to a client's own `UserState`
  or a server-side cascade such as a mute-by-admin or ACL change): only the fields
  the client explicitly sent, plus any fields the server's cascade logic
  synthesized (e.g. unmuting synthesizes an explicit `self_deaf=false` if the user
  was deafened), are echoed back. Untouched fields — including unrelated voice
  flags — are omitted, not re-sent as `false`.

Sending every voice flag (including `false` ones) on every broadcast was tried and
reverted: the official client treats each explicitly-present field as a discrete
event, so observers would log a burst of spurious "Muted." / "Recording stopped" /
etc. messages (including on Mumla join announces). Mumla/Humla need the unmute
echo to carry explicit `self_mute`/`self_deaf` presence, but they do not
rebroadcast the server's full `UserState` — see
[0007-userstate-field-presence.md](../features/0007-userstate-field-presence.md).

**Cascade (murmur parity):** deafened implies muted. Setting `self_deaf`/`deaf` to
true forces the matching mute flag; clearing mute clears deaf; clearing deaf alone
leaves mute set. Clients that want "unmute on undeaf" must send both fields
(Mumla's UI already does). The cascade logic mutates the inbound `UserState`
message in place (setting the synthesized field's value and its `SetFields`
presence bit) so the same message that is validated is the one broadcast.

| Field | Type | Description |
|-------|------|-------------|
| `session` | uint32 | User session ID (always present on snapshots and server echoes) |
| `actor` | uint32 | Session of the user making changes (delta echoes) |
| `name` | string | Username |
| `user_id` | uint32 | Registered user ID |
| `channel_id` | uint32 | Current channel (always present on **snapshots**, including root = 0; omitted from mute-toggle **deltas** unless a move was requested) |
| `mute` | bool | Server-muted (requires MuteDeafen to set) |
| `deaf` | bool | Server-deafened (requires MuteDeafen; implies mute) |
| `suppress` | bool | Suppressed (mirrors lack of Speak ACL; clients may only clear) |
| `self_mute` | bool | Self-muted (self only) |
| `self_deaf` | bool | Self-deafened (self only; implies self_mute) |
| `texture` | bytes | User avatar |
| `plugin_context` | bytes | Plugin positional audio context (applied server-side; never broadcast — a plugin-only message is a no-op on the wire) |
| `plugin_identity` | string | Plugin identity (applied server-side; never broadcast) |
| `comment` | string | User comment (HTML) |
| `hash` | string | Certificate hash |
| `comment_hash` | bytes | SHA-1 hash of comment |
| `texture_hash` | bytes | SHA-1 hash of texture |
| `priority_speaker` | bool | Priority speaker flag (cleared on channel switch) |
| `recording` | bool | Currently recording (self only; announced via TextMessage) |
| `temporary_access_tokens` | string[] | Additional access tokens |
| `listening_channel_add` | uint32[] | Channels to start listening to |
| `listening_channel_remove` | uint32[] | Channels to stop listening to |
| `listening_volume_adjustment` | ListeningVolumeAdjustment[] | Per-channel volume for listeners |

### Type 10 — BanList

Request or update the server ban list.

| Field | Type | Description |
|-------|------|-------------|
| `bans` | BanEntry[] | Ban entries |
| `query` | bool | True = request list; false = replace list |

**BanEntry:**

| Field | Type | Description |
|-------|------|-------------|
| `address` | bytes | IP address (4 or 16 bytes) |
| `mask` | uint32 | Subnet mask bits |
| `name` | string | Banned username |
| `hash` | string | Certificate hash |
| `reason` | string | Ban reason |
| `start` | string | Ban start time |
| `duration` | uint32 | Duration in seconds (0 = permanent) |

### Type 11 — TextMessage

| Field | Type | Description |
|-------|------|-------------|
| `actor` | uint32 | Sender session (set by server) |
| `session` | uint32[] | Target user sessions (private message) |
| `channel_id` | uint32[] | Target channels |
| `tree_id` | uint32[] | Target channel trees (recursive) |
| `message` | string | Message content (HTML) |

### Type 12 — PermissionDenied

Sent by the server when a client lacks permission.

| Field | Type | Description |
|-------|------|-------------|
| `permission` | uint32 | The denied permission |
| `channel_id` | uint32 | Relevant channel |
| `session` | uint32 | Relevant user session |
| `reason` | string | Human-readable reason |
| `type` | DenyType | Denial category |
| `name` | string | Related name |

**DenyType enum:** Text, Permission, SuperUser, ChannelName, TextTooLong, H9K (HTML too long), TemporaryChannel, MissingCertificate, UserName, ChannelFull, NestingLimit, ChannelCountLimit, ChannelListenerLimit, UserListenerLimit.

### Type 13 — ACL

Request or update ACLs for a channel.

| Field | Type | Description |
|-------|------|-------------|
| `channel_id` | uint32 | Target channel |
| `inherit_acls` | bool | Whether to inherit from parent |
| `groups` | ChanGroup[] | Group definitions |
| `acls` | ChanACL[] | ACL entries |
| `query` | bool | True = request; false = update |

### Type 14 — QueryUsers

Look up user IDs by name or names by ID.

| Field | Type | Description |
|-------|------|-------------|
| `ids` | uint32[] | User IDs to look up |
| `names` | string[] | Usernames to look up |

### Type 15 — CryptSetup

Key exchange for UDP encryption. Sent by the server during connection setup.

| Field | Type | Description |
|-------|------|-------------|
| `key` | bytes | 128-bit AES key (16 bytes) |
| `client_nonce` | bytes | Client→server nonce (16 bytes) |
| `server_nonce` | bytes | Server→client nonce (16 bytes) |

See [encryption.md](encryption.md).

### Type 16 — ContextActionModify

Add or remove context menu actions for a client.

| Field | Type | Description |
|-------|------|-------------|
| `action` | string | Action identifier |
| `text` | string | Display text |
| `context` | uint32 | Context flags (Server=1, Channel=2, User=4) |
| `operation` | Operation | Add or Remove |

### Type 17 — ContextAction

Client triggered a context action.

| Field | Type | Description |
|-------|------|-------------|
| `session` | uint32 | Target user session (if user context) |
| `channel_id` | uint32 | Target channel (if channel context) |
| `action` | string | Action identifier |

### Type 18 — UserList

List or modify registered users.

| Field | Type | Description |
|-------|------|-------------|
| `users` | UserListEntry[] | Registered users |

### Type 19 — VoiceTarget

Configure a whisper target for the sending client.

| Field | Type | Description |
|-------|------|-------------|
| `id` | uint32 | Target ID (1–30) |
| `targets` | Target[] | Target definitions |

**Target:**

| Field | Type | Description |
|-------|------|-------------|
| `session` | uint32[] | Direct user sessions |
| `channel_id` | uint32 | Target channel |
| `group` | string | ACL group filter |
| `links` | bool | Include linked channels |
| `children` | bool | Include sub-channels |

### Type 20 — PermissionQuery

Request or report permissions for a channel.

| Field | Type | Description |
|-------|------|-------------|
| `channel_id` | uint32 | Channel |
| `permissions` | uint32 | Permission bitmask |
| `flush` | bool | Flush cached permissions |

### Type 21 — CodecVersion

Server broadcasts the preferred codec.

| Field | Type | Description |
|-------|------|-------------|
| `alpha` | int32 | CELT Alpha version |
| `beta` | int32 | CELT Beta version |
| `prefer_alpha` | bool | Prefer Alpha over Beta |
| `opus` | bool | Server supports Opus |

### Type 22 — UserStats

Request or report detailed user statistics.

| Field | Type | Description |
|-------|------|-------------|
| `session` | uint32 | Target user session |
| `stats_only` | bool | Only basic stats, no certificates |
| `certificates` | bytes[] | User's certificate chain (DER) |
| `from_client` | Stats | Client→server stats |
| `from_server` | Stats | Server→client stats |
| `udp_packets` | uint32 | UDP packets |
| `tcp_packets` | uint32 | TCP packets |
| `udp_ping_avg` | float | UDP ping average |
| `udp_ping_var` | float | UDP ping variance |
| `tcp_ping_avg` | float | TCP ping average |
| `tcp_ping_var` | float | TCP ping variance |
| `version` | Version | Client version |
| `celt_versions` | int32[] | CELT versions |
| `address` | bytes | Client IP (for admins) |
| `bandwidth` | uint32 | Current bandwidth |
| `onlinesecs` | uint32 | Seconds connected |
| `idlesecs` | uint32 | Seconds idle |
| `strong_certificate` | bool | Strong certificate |
| `opus` | bool | Opus support |

### Type 23 — RequestBlob

Request large data (textures, comments, descriptions) by hash.

| Field | Type | Description |
|-------|------|-------------|
| `session_texture` | uint32[] | User sessions for texture blobs |
| `session_comment` | uint32[] | User sessions for comment blobs |
| `channel_description` | uint32[] | Channel IDs for description blobs |

### Type 24 — ServerConfig

Server configuration broadcast.

| Field | Type | Description |
|-------|------|-------------|
| `max_bandwidth` | uint32 | Max bandwidth per user (bps) |
| `welcome_text` | string | Welcome message (HTML) |
| `recording_allowed` | bool | Whether recording is allowed |
| `allow_html` | bool | Whether HTML in messages is allowed |
| `message_length` | uint32 | Max text message length |
| `image_message_length` | uint32 | Max image message length |
| `max_users` | uint32 | Max users |

### Type 25 — SuggestConfig

Server suggests client configuration.

| Field | Type | Description |
|-------|------|-------------|
| `version_v1` | uint32 | Suggested client version |
| `version_v2` | uint64 | Suggested client version (v2) |
| `positional` | bool | Suggest positional audio |
| `push_to_talk` | bool | Suggest push-to-talk |

### Type 26 — PluginDataTransmission

Plugin data relay between clients.

| Field | Type | Description |
|-------|------|-------------|
| `senderSession` | uint32 | Sender (set by server) |
| `receiverSessions` | uint32[] | Target sessions |
| `data` | bytes | Plugin data payload |
| `dataID` | string | Plugin data identifier |

## Reference

- Protobuf source: `research/mumble/src/Mumble.proto`
- gumble handler table: `research/gumble/gumble/handlers.go`

## Schema conformance

`pkg/mumble/protocol/messages/schema_lint_test.go` cross-checks every message
struct in this catalog against a vendored copy of the upstream schema
(`pkg/mumble/protocol/messages/testdata/Mumble.proto`): every implemented Go
field must map to a real proto field at the right number and wire type, every
proto field we don't implement must be an explicitly tracked gap, and every
`Marshal()` is probed byte-for-byte to confirm it actually emits what the
mapping claims. See [protocol-encoding.md](../architecture/protocol-encoding.md#schema-conformance-lint).
