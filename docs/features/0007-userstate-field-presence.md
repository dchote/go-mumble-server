# 0007: UserState Field Presence and Voice State

## Status: Implemented

Resolves [#1](https://github.com/dchote/go-mumble-server/issues/1) and its
[regression](https://github.com/dchote/go-mumble-server/issues/1#issuecomment-5077919135).

## Summary

`UserState` broadcasts follow Murmur's actual wire behavior: a one-time **snapshot**
shape for full user syncs (only `true` voice flags, with `deaf`/`mute` and
`self_deaf`/`self_mute` mutually exclusive), and a minimal **delta echo** shape for
everything else (only the fields the client sent, plus whatever the server's
cascade logic synthesized on top). Self-mute/self-deaf cascade, admin mute/deaf
cascade, Suppress sync from the Speak ACL, recording announcements, and channel-move
side effects (priority speaker clear, suppress resync) all mutate and re-broadcast
the same message in place, matching `Server::msgUserState` in upstream Murmur.

**Note on issue #1's title:** the report claimed undeafen must clear self-mute.
Upstream Murmur's invariant is the opposite asymmetry — **deafened implies muted**;
un-muting clears deafen, but un-deafening leaves mute alone. Clients that want
"unmute on undeaf" send both fields explicitly. Mumla's own UI already does this:
its deafen button calls `setSelfMuteDeafState(deafened, deafened)`, and its mute
button clears deafen when unmuting (`deafened &= muted`). This server matches
Murmur; Mumla's "undeafen unmutes" behaviour is a client-side convention, not a
wire-level cascade.

## How Mumla / Humla actually behave

Mumla does not parse protobuf itself. All UserState handling lives in
[Humla](https://github.com/Morlunk/Humla) (`research/humla`):

| Layer | Behaviour |
| --- | --- |
| Send (`HumlaService.setSelfMuteDeafState`) | Always sends **both** `self_mute` and `self_deaf`. Never sends admin `mute`/`deaf`/`suppress`/`recording`/`priority_speaker` on a self-toggle. |
| Model (`ModelHandler.messageUserState`) | Proto2 has-bits: updates each flag only when present; absent leaves local state unchanged. No optimistic local update on toggle — UI waits for the server echo. |
| Audio (`AudioHandler.messageUserState`) | If any of `mute`/`self_mute`/`suppress` is present for self, recomputes TX mute as `getMute() \|\| getSelfMute() \|\| getSuppress()`, treating **absent** fields as `false`. This is a known Humla bug (not presence-safe). |
| `PermissionDenied` | UI dialog / notification only — **does not disconnect**. |

"Echo-driven" for Mumla therefore means: **local mute UI and TX gate are derived
from the server's reply to the client's own toggle**, not that Mumla rebroadcasts
the server's full `UserState`. Over-broadcasting admin `false` flags does not
cause Mumla to re-send those fields or trip `PermissionDenied`.

What Mumla needs from the server to unmute correctly:

1. Echo a `UserState` with `hasSelfMute` / `hasSelfDeaf` set to the new values
   (Humla always sent both; the echo must carry them with explicit presence).
2. Prefer true deltas — omit unchanged voice flags — so official-client observers
   don't log a burst of spurious per-field events on every toggle or join.

## History

This feature went through two rounds because the first fix, while it solved the
reported bug, over-corrected in a way that broke different clients. Both rounds
are documented here so the reasoning isn't lost.

### Round 1 — the original bug

Mumla and Plumble users could mute themselves but never unmute; the microphone and
speaker state stayed locked for the rest of the session. The official Mumble client
was unaffected.

`Mumble.proto` is `syntax = "proto2"`, so every `UserState` field has explicit
presence: an assigned `false` is still written to the wire, and "absent" is a
distinct state from "false". The hand-written `UserState.Marshal` used proto3-style
implicit presence (`if m.SelfMute { write }`), so every unmute broadcast dropped the
cleared flag. The official client applies mute state locally and never notices;
Mumla and Plumble derive their mute icon from the server's echo, so a dropped
`self_mute=false` left them muted with no way to recover.

The first fix added a `SetFields` has-bit mask and made authoritative broadcasts
set presence bits for **all seven voice flags** on every `UserState`, so cleared
flags would always be visible.

### Round 2 — the regression

Setting all seven voice flags on every broadcast fixed Mumla/Plumble unmute but
broke almost everything else. The official client started logging a burst of
spurious per-field change messages ("Muted.", "Recording stopped", "You assumed
priority speaker status.", "You were unsuppressed.") on every `UserState` —
including on Mumla join announces — because it treats a field's mere *presence*
as a discrete event, not a value refresh. See the
[reporter's log](https://github.com/dchote/go-mumble-server/issues/1#issuecomment-5077919135).

Root-causing this required going back to Murmur's actual C++ source
(`research/mumble/src/murmur/Messages.cpp`, `msgUserState`) rather than re-deriving
the behavior from first principles. Murmur does **not** treat "proto2 has explicit
presence" as license to always send every field. It sends two distinct shapes:

- A **snapshot** — built once per user, during login sync or when announcing a
  newly joined user — that only sets fields with non-default values (voice flags
  are omitted unless true, and `deaf`/`self_deaf` implying `mute`/`self_mute`
  means the implied field is left unset).
- A **delta echo** — the reply to a client's own `UserState`, or the broadcast of
  a server-side cascade — that only carries the fields the triggering message
  touched (as sent by the client, or as synthesized by cascade logic like
  admin-mute-implies-deaf-clear-on-unmute), copying the *same* message object
  in-place rather than re-deriving a fresh snapshot.

An independent second data point came from reviewing `libmumble`
(`research/libmumble`): its serializer had the exact same "always send every
optional field" bug we'd just introduced, and its parsing code showed the same
presence-vs-absence asymmetry clients must handle. This confirmed the diagnosis
was a general protocol property, not something specific to our Go implementation.

### Round 2 fix

- `userToState` (snapshot construction) now only sets a voice flag when it's
  `true`, with `else if` exclusivity matching Murmur: a deafened user's snapshot
  carries `deaf=true` and omits `mute` (implied); a self-deafened user's snapshot
  carries `self_deaf=true` and omits `self_mute`.
- `handleUserState` no longer builds a fresh outgoing message. It mutates the
  client's inbound `UserState` in place — `applySelfVoiceState` /
  `applyAdminVoiceState` set synthesized fields' values *and* their `SetFields`
  presence bits directly on that message — and broadcasts the mutated message
  unchanged. Fields the client didn't send and the cascade didn't touch are
  never added.
- Server-initiated minimal broadcasts (`RefreshSuppressStates` after an ACL edit,
  `MuteSession` from the REST API, per-user moves in `handleChannelRemove`) send
  `{session, <changed fields only>}` rather than a full snapshot, matching
  Murmur's `clearACLCache`-driven resends.
- `plugin_context` / `plugin_identity` are applied server-side but never set
  Murmur's `bBroadcast` on their own; a plugin-only message produces no UserState
  broadcast (not even a bare `{session, actor}`).
- Unimplemented fields (`UserID`, `TemporaryAccessTokens`,
  `ListeningChannelAdd/Remove/VolumeAdjustment`) are stripped before apply/echo.
- `has_name()` on an inbound `UserState` is unconditionally denied; a `channel_id`
  naming an unknown or unchanged channel is silently dropped.

## Wire encoding — `pkg/mumble/protocol/messages/user.go`

- `SetFields` is a bidirectional proto2 has-bit mask: `Unmarshal` records which
  fields the peer actually sent; `Marshal` emits a field when `Has(bit) ||
  <non-default>` — so an explicitly-set default (e.g. cascade-synthesized
  `self_mute=false`) survives encoding, but fields nobody touched stay absent.
- There is deliberately no `UserStateVoiceFields` "set everything" constant
  anymore; the whole point of round 2 was that broadcasting every voice flag
  unconditionally is the bug, not the fix.

## Cascade — `internal/mumble/voicestate.go`

| Inbound | Resulting state |
| --- | --- |
| `self_deaf=true` | deaf and muted (mute implied) |
| `self_deaf=false` | undeafened, **still muted** |
| `self_mute=false` | unmuted and undeafened |
| `self_mute=true` | muted only |

Same pattern for administrative `mute` / `deaf`. `applySelfVoiceState` and
`applyAdminVoiceState` mutate the caller-supplied `*messages.UserState` in place
(setting both the field value and its `SetFields` bit for any synthesized change),
so the message that gets validated is the exact message that gets broadcast.
`applyChannelEnterEffects` centralizes the on-channel-move side effects (clear
priority speaker; resync `Suppress` from the Speak ACL), taking a pre-resolved
`maySpeak bool` so it never needs to acquire ACL locks while holding the user
manager's lock (see "Concurrency" below). REST `MuteSession` shares the same
cascade helper as the TCP handler.

## Handler behaviour — `internal/mumble/handlers.go`

- No-op messages (no handled fields) do not broadcast.
- Messages with **broadcast-worthy** handled fields always broadcast (Murmur
  broadcasts when a field is present, even if the value is unchanged — Humla needs
  the echo to update local mute state). Plugin-only messages are applied but do
  not broadcast.
- Self-only fields aimed at another session are dropped.
- Admin voice fields always require `MuteDeafen` (including self-target), matching
  Murmur.
- Channel enter: clear priority speaker; sync `Suppress` from the Speak ACL;
  self-moves also receive a `PermissionQuery` for the destination channel.
- ACL edits via REST: `RefreshSuppressStates` recomputes Suppress for all sessions
  and sends a minimal `{session, suppress}` delta per affected user.
- Recording toggles announce via a root-tree `TextMessage`, but only to clients
  whose negotiated protocol version predates 1.2.3 (`clientVersionFull` /
  `connection.Conn.ClientVersion`); newer clients render the recording indicator
  from `UserState.recording` directly and don't need the text announcement.

## Concurrency

Resolving ACL permissions (`maySpeak`) requires walking the channel/group graph,
which can call back into the user manager (e.g. to enumerate group members). Doing
that resolution *inside* `user.Manager.UpdateUser`'s callback — while already
holding that manager's lock — is a lock-ordering cycle waiting to happen. The fix
is structural: `Server.maySpeak` is the single helper every caller uses, and it is
always invoked **before** `UpdateUser`. The precomputed boolean is then passed into
the locked callback. This surfaced as a real deadlock once the round 2 regression
tests started exercising channel moves and ACL refreshes end-to-end.

## Residual client risk: Humla AudioHandler vs suppress-only deltas

Humla's `AudioHandler` recomputes TX mute as
`getMute() || getSelfMute() || getSuppress()` whenever *any* of those three fields
is present, treating absent ones as `false`. A murmur-shaped `{session, suppress}`
delta (from `RefreshSuppressStates` or a channel-move suppress flip) can therefore
briefly unlock Humla TX audio while the model layer still shows self-muted.

We deliberately still send Murmur's minimal suppress delta rather than
over-broadcasting the current `self_mute` to paper over that client bug — the
same over-broadcast is what caused the round 2 regression. The correct fix lives
in Humla (make `AudioHandler` presence-aware, matching `ModelHandler`).

## Schema conformance lint

`pkg/mumble/protocol/messages/schema_lint_test.go`, backed by a vendored copy of
upstream `Mumble.proto` at `pkg/mumble/protocol/messages/testdata/Mumble.proto`,
guards against this class of bug recurring for any message, not just `UserState`:
it validates that every implemented field's number and wire type match the schema,
that every unimplemented field is a tracked, deliberate gap, and that `Marshal()`
actually emits what the mapping tables claim. See
[protocol-encoding.md](../architecture/protocol-encoding.md#schema-conformance-lint).

## Client compatibility

| Client | Expected behaviour after this change |
| --- | --- |
| Official Mumble | Unmute/undeafen works; no spurious per-field log spam from over-broadcast snapshots or mute-toggle echoes |
| Mumla / Plumble (Humla) | Unmute/undeafen echoes carry the `self_mute`/`self_deaf` the client sent (with explicit presence); join announces omit false voice flags |
| Other presence-aware clients | Same — clears are visible on the fields that actually changed; unrelated flags stay absent |

## Files modified

- `pkg/mumble/protocol/messages/user.go`, `user_test.go`
- `pkg/mumble/protocol/messages/schema_lint_test.go`,
  `pkg/mumble/protocol/messages/testdata/Mumble.proto` (new)
- `pkg/mumble/user.go` (round 1)
- `internal/user/manager.go` (round 1)
- `internal/mumble/voicestate.go`, `voicestate_test.go`
- `internal/mumble/handlers.go`, `handlers_userstate_test.go`
- `internal/connection/conn.go` — client protocol version tracking
- `internal/server/server.go` — ACL change hooks Suppress refresh (round 1)
- `docs/architecture/protocol-encoding.md`
- `docs/protocol/control-messages.md`
- `docs/product-overview.md`
- `docs/patterns/channel-tree-pattern.md`
- `docs/patterns/connection-lifecycle-pattern.md`
- `docs/features/0001-api-frontend-servers-channels-config-db.md`
- `README.md`

## Tests

- Wire round-trip for explicit presence on all seven voice flags
  (`pkg/mumble/protocol/messages/user_test.go`).
- Snapshot construction: only `true` flags present, `else if` exclusivity
  (`TestUserToState_OmitsFalseVoiceFlags`,
  `TestUserToState_EmitsTrueVoiceFlagsWithElseIfExclusivity`).
- Cascade tables for self and admin mute/deaf, asserting in-place mutation of the
  inbound message (`TestApplySelfVoiceState_*`, `TestApplyAdminVoiceState_*`).
- End-to-end `handleUserState` over a real socket
  (`internal/mumble/handlers_userstate_test.go`):
  - Humla-shaped unmute (`self_mute`+`self_deaf` both false, both present)
  - Humla-shaped mute toggle echo carries only those two voice flags
  - undeafen-alone leaves mute; does not invent a `self_mute` presence bit
  - join announce (`Broadcast(joiner, userToState(joiner))`) omits every false
    voice flag (exact issue #1 regression shape)
  - plugin-only messages apply server-side but do not broadcast
  - recording TextMessage gated to clients `< 1.2.3`
  - same-value recording resend is a no-op; renames denied; same-channel moves
    drop the whole message; channel moves echo flipped suppress/priority;
    `RefreshSuppressStates`/`MuteSession` send minimal deltas
- Schema conformance lint (`pkg/mumble/protocol/messages/schema_lint_test.go`)
  validates every message's field numbers/wire types against the vendored schema.
- Verified with `CGO_ENABLED=1 go test -race -timeout=60s ./...`.

## Compatibility notes

- **Wire**: `UserState` broadcasts are smaller on average than round 1's
  always-seven-flags approach; snapshots only include true flags, and echoes only
  include changed fields. This is a net reduction in bytes sent and in spurious
  client-side events, not a breaking change.
- **Behaviour**: self-targeted administrative `mute`/`deaf` still require
  `MuteDeafen` (clients use `self_mute`/`self_deaf` for their own state).
- No REST schema or frontend changes.
