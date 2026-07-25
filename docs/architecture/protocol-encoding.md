# Protocol Encoding

> **Do not use Google protobuf.** This project uses native Go structs and hand-written wire encoding for all Mumble protocol messages.

## Design Decision

go-mumble-server implements the Mumble protocol **without** `google.golang.org/protobuf` or any protoc-generated code. Rationale:

- **Zero protobuf dependency** — No build-time protoc, no generated `.pb.go` files, no `research/mumble` submodule for proto sources
- **Native Go** — Message types are plain Go structs in `pkg/mumble/protocol/messages/`
- **Hand-written wire encoder** — `pkg/mumble/protocol/wire/` produces Mumble-compatible binary (same byte layout as the upstream protobuf spec, for client compatibility)
- **Maintainability** — All protocol code is in Go; no external codegen step

## Field presence is explicit (proto2)

`Mumble.proto` declares `syntax = "proto2"` and every field is `optional`, which means **explicit presence**. This is the single most important detail to get right in a hand-written encoder:

- A field that has been *assigned* is written to the wire even when its value is the type default. `set_self_mute(false)` produces bytes.
- "Absent" and "false" are therefore **different states**. Absent means "unchanged / unknown"; `false` means "definitively off".

A proto3-style `if value != default { write }` encoder silently collapses these two states, and the resulting bug is subtle: it only affects peers that interpret absence as "no information". Real clients differ here. The official Mumble client applies its own mute state locally before telling the server, so it never notices. Mumla and Plumble derive their mute state from the server's echo, so a dropped `self_mute=false` leaves them muted with no way to recover. See [0007-userstate-field-presence.md](../features/0007-userstate-field-presence.md).

Messages that need presence carry a `SetFields` bitmask of has-bits, used in **both** directions: `Unmarshal` records which fields the peer sent, and `Marshal` uses it to emit explicitly-set defaults. When adding a bool or scalar to a message where "explicitly off" is meaningful, give it a has-bit rather than relying on the zero value.

## Explicit presence is not "always send every field"

A tempting but wrong reading of "proto2 has explicit presence" is "so the server
should always send every field, including explicit `false`s, so clients always
have the full picture." We tried this for `UserState` and had to revert it: real
clients treat a field's *presence* in the message as a change notification, not
as a value refresh. The official Mumble client logs every explicitly-present
field as an event ("Muted.", "Recording stopped", "You assumed priority speaker
status."). Mumla/Plumble (Humla) are "echo-driven" in a different sense: they
derive local mute UI from the server's reply to their own toggle (and do not
optimistically update), so an unmute echo must carry explicit `self_mute`/
`self_deaf` presence — but they do **not** rebroadcast the server's full
`UserState`, and over-broadcast admin `false` flags do not cause them to send
`PermissionDenied`-triggering fields. See
[0007-userstate-field-presence.md](../features/0007-userstate-field-presence.md).

Murmur's actual rule, which this server now follows for `UserState`:

- **Snapshots** (one-time full sync of a user, e.g. on login or when another user
  joins) include only the voice flags that are `true`, with `deaf`/`mute` and
  `self_deaf`/`self_mute` mutually exclusive (deaf implies mute, so mute is
  omitted).
- **Delta echoes** (the response to a client-initiated `UserState`, or a
  server-side cascade like an admin mute or ACL change) include only the fields
  the client actually sent, plus whatever the cascade logic synthesized on top —
  never a full re-snapshot of unrelated fields.

This same distinction generalizes to every message that echoes client input:
prefer emitting the minimal delta a real Murmur would emit over broadcasting a
full state dump, even when a full dump would be easier to write. See
[control-messages.md](../protocol/control-messages.md#type-9--userstate) and
[0007-userstate-field-presence.md](../features/0007-userstate-field-presence.md).

## Schema conformance lint

`pkg/mumble/protocol/messages/schema_lint_test.go` is a standing regression test
against protocol drift. It vendors a copy of the upstream schema at
`pkg/mumble/protocol/messages/testdata/Mumble.proto` (not compiled — just parsed
by a small scanner in the test) and validates three things for every message
struct in this package:

1. Every exported Go struct field is explicitly classified against a proto
   field name, or marked as a synthetic/Go-only field (has-bits, presence
   flags, or a documented non-upstream extension like `Version.CryptoModes`).
2. Every proto field we don't implement is a named, tracked entry in an
   allowlist — if upstream adds a new field to a message we implement, the lint
   fails until that field is deliberately triaged (implemented, or added to the
   allowlist with a reason).
3. For every top-level message, a fixture that populates every implemented
   field is marshaled and scanned tag-by-tag, confirming the field numbers and
   wire types we actually emit match the schema — catching a real encoding bug,
   not just a stale mapping table.

When upstream `Mumble.proto` changes, re-vendor the testdata copy (see the
header comment in that file for instructions) and run
`go test ./pkg/mumble/protocol/messages/...`; failures point at exactly what
changed and needs a decision (implement it, or allowlist the gap).

## For Contributors

When adding or changing protocol messages:

1. **Do not add** `google.golang.org/protobuf` to `go.mod`
2. **Do not add** protoc, `protoc-gen-go`, or any `.proto` generation scripts as build requirements
3. **Use** the existing `pkg/mumble/protocol/messages/` structs and `pkg/mumble/protocol/wire/` encoder
4. **Reference** `research/mumble/src/Mumble.proto` and `MumbleUDP.proto` only as documentation for field numbers and wire format — we do not compile them. A copy of `Mumble.proto` is vendored (not compiled) at `pkg/mumble/protocol/messages/testdata/Mumble.proto` specifically for the schema conformance lint described below.
5. **Preserve field presence** — do not add an `if value != default` encode guard to a field where an explicit default is meaningful; gate on the has-bit as well
6. **Prefer minimal deltas over full re-snapshots** when echoing client-initiated state changes — see "Explicit presence is not 'always send every field'" above
7. **Update `schema_lint_test.go`** when adding, removing, or renumbering a message field — either add a `schemaFieldMap` entry or a documented `unimplementedFields` gap; the lint fails loudly if you forget

The upstream Mumble protocol uses protobuf wire format on the wire. Our encoder replicates that format by hand so we remain wire-compatible with all Mumble clients while avoiding the protobuf library.
