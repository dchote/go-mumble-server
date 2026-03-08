# Protocol Encoding

> **Do not use Google protobuf.** This project uses native Go structs and hand-written wire encoding for all Mumble protocol messages.

## Design Decision

go-mumble-server implements the Mumble protocol **without** `google.golang.org/protobuf` or any protoc-generated code. Rationale:

- **Zero protobuf dependency** — No build-time protoc, no generated `.pb.go` files, no `research/mumble` submodule for proto sources
- **Native Go** — Message types are plain Go structs in `pkg/mumble/protocol/messages/`
- **Hand-written wire encoder** — `pkg/mumble/protocol/wire/` produces Mumble-compatible binary (same byte layout as the upstream protobuf spec, for client compatibility)
- **Maintainability** — All protocol code is in Go; no external codegen step

## For Contributors

When adding or changing protocol messages:

1. **Do not add** `google.golang.org/protobuf` to `go.mod`
2. **Do not add** protoc, `protoc-gen-go`, or any `.proto` generation scripts as build requirements
3. **Use** the existing `pkg/mumble/protocol/messages/` structs and `pkg/mumble/protocol/wire/` encoder
4. **Reference** `research/mumble/src/Mumble.proto` and `MumbleUDP.proto` only as documentation for field numbers and wire format — we do not compile them

The upstream Mumble protocol uses protobuf wire format on the wire. Our encoder replicates that format by hand so we remain wire-compatible with all Mumble clients while avoiding the protobuf library.
