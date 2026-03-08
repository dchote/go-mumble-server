#!/bin/bash
set -e
cd "$(dirname "$0")/.."

PROTO_SRC="research/mumble/src"
PROTO_OUT="pkg/mumble/proto"

if ! command -v protoc &> /dev/null; then
    echo "Error: protoc not found. Install Protocol Buffers compiler."
    echo "  macOS:  brew install protobuf"
    echo "  Linux:  apt install protobuf-compiler"
    exit 1
fi

if ! command -v protoc-gen-go &> /dev/null; then
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

mkdir -p "${PROTO_OUT}"

echo "Generating Go protobuf from Mumble.proto..."
protoc \
    --proto_path="${PROTO_SRC}" \
    --go_out="${PROTO_OUT}" \
    --go_opt=paths=source_relative \
    "${PROTO_SRC}/Mumble.proto"

echo "Generating Go protobuf from MumbleUDP.proto..."
protoc \
    --proto_path="${PROTO_SRC}" \
    --go_out="${PROTO_OUT}" \
    --go_opt=paths=source_relative \
    "${PROTO_SRC}/MumbleUDP.proto"

echo "Done. Generated files in ${PROTO_OUT}/"
