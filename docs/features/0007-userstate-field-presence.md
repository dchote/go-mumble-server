# 0007: UserState Field Presence and Voice State

## Status: Implemented

Resolves [#1](https://github.com/dchote/go-mumble-server/issues/1).

## Summary

Outgoing `UserState` messages encode proto2 explicit field presence for the voice
flags, so a flag being *cleared* is transmitted rather than omitted. Self-mute /
self-deaf cascade, Suppress sync from Speak ACL, recording announcements, voice
routing gates, and locking around voice flags follow upstream murmur.

**Note on issue #1's title:** the report claimed undeafen must clear self-mute.
Upstream murmur's invariant is the opposite asymmetry — **deafened implies muted**;
un-muting clears deafen, but un-deafening leaves mute alone. Clients that want
"unmute on undeaf" send both fields explicitly (the official client has a setting
for this). This server matches murmur.

## Problem

Mumla and Plumble users could mute themselves but never unmute. The microphone and
speaker state stayed locked for the rest of the session. The official Mumble client
was unaffected.

`Mumble.proto` is `syntax = "proto2"`, so every `UserState` field has explicit
presence: an assigned `false` is still written to the wire, and "absent" is a
distinct state from "false". The hand-written `UserState.Marshal` used proto3-style
implicit presence (`if m.SelfMute { write }`), so every unmute broadcast dropped the
cleared flags. Echo-driven clients never learned they were unmuted.

## Changes

### Wire encoding — `pkg/mumble/protocol/messages/user.go`

- `SetFields` is a bidirectional proto2 has-bit mask.
- `Marshal` emits a field when `Has(bit) || <non-default>` (including `session` and
  `channel_id`; root `channel_id=0` requires the has-bit so client mute toggles that
  omit those fields are not misread as “move to root / session 0”).
- Authoritative broadcasts set `UserStateVoiceFields` plus session/channel has-bits
  so cleared flags stay visible.

### Voice state locking — `pkg/mumble/user.go`, `internal/user/manager.go`

- `mumble.VoiceState` embedded in `mumble.User`.
- `Snapshot` / `UpdateUser` return **deep** copies (texture, plugin context, tokens).
- Hot path uses `SpeakGateFor` / `VoiceState` / `ChannelID` (no full-user copy per packet).

### Cascade — `internal/mumble/voicestate.go`

| Inbound | Resulting state |
| --- | --- |
| `self_deaf=true` | deaf and muted (mute implied) |
| `self_deaf=false` | undeafened, **still muted** |
| `self_mute=false` | unmuted and undeafened |
| `self_mute=true` | muted only |

Same pattern for administrative `mute` / `deaf`. REST `MuteSession` shares the helper.

### Handler behaviour — `internal/mumble/handlers.go`

- No-op messages (no handled fields) do not broadcast.
- Messages with handled fields always broadcast (murmur broadcasts when a field is
  present, even if the value is unchanged — echo-driven clients need the echo).
- Self-only fields aimed at another session are dropped.
- Admin voice fields always require `MuteDeafen` (including self-target), matching murmur.
- Channel enter: clear priority speaker; sync `Suppress` from Speak ACL.
- ACL edits via REST: `RefreshSuppressStates` recomputes Suppress for all sessions.
- Recording toggles announce via root-tree `TextMessage` (recording is always allowed;
  murmur's optional disallow/kick is not configured here yet).

## Client compatibility

| Client | Expected behaviour after this change |
| --- | --- |
| Official Mumble | Unchanged (local-first mute UI) |
| Mumla / Plumble | Unmute/undeafen echoes carry explicit `false`; mic unlocks |
| Other echo-driven clients | Same as Mumla — presence bits make clears visible |

## Files modified

- `pkg/mumble/protocol/messages/user.go`, `user_test.go`
- `pkg/mumble/user.go`
- `internal/user/manager.go`
- `internal/mumble/voicestate.go`, `voicestate_test.go`
- `internal/mumble/handlers.go`, `handlers_userstate_test.go`
- `internal/server/server.go` — ACL change hooks Suppress refresh
- `docs/architecture/protocol-encoding.md`
- `docs/protocol/control-messages.md`
- `docs/technical-overview.md`

## Tests

- Wire round-trip for explicit `false` on all seven voice flags.
- Cascade tables for self and admin mute/deaf.
- End-to-end `handleUserState` over a real socket: unmute visible, deafen⇒mute,
  undeafen leaves mute with explicit `self_mute=true` on the wire, suppress-assert
  denied, empty no-op does not broadcast, MuteSession unmute clears deaf.
- Verified with `CGO_ENABLED=1 go test -race -timeout=60s ./...`.

## Compatibility notes

- **Wire**: additive (extra explicit-false bools on broadcasts).
- **Behaviour**: self-targeted administrative `mute`/`deaf` now need `MuteDeafen`
  (clients use `self_mute`/`self_deaf` for their own state).
- No REST schema or frontend changes.
