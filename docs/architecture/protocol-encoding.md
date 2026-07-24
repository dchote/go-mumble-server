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

## For Contributors

When adding or changing protocol messages:

1. **Do not add** `google.golang.org/protobuf` to `go.mod`
2. **Do not add** protoc, `protoc-gen-go`, or any `.proto` generation scripts as build requirements
3. **Use** the existing `pkg/mumble/protocol/messages/` structs and `pkg/mumble/protocol/wire/` encoder
4. **Reference** `research/mumble/src/Mumble.proto` and `MumbleUDP.proto` only as documentation for field numbers and wire format — we do not compile them
5. **Preserve field presence** — do not add an `if value != default` encode guard to a field where an explicit default is meaningful; gate on the has-bit as well

The upstream Mumble protocol uses protobuf wire format on the wire. Our encoder replicates that format by hand so we remain wire-compatible with all Mumble clients while avoiding the protobuf library.
