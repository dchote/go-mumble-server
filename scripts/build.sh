#!/bin/bash
set -e
cd "$(dirname "$0")/.."

VERSION="${VERSION:-dev}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}"
BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS="-s -w"
LDFLAGS="${LDFLAGS} -X main.version=${VERSION}"
LDFLAGS="${LDFLAGS} -X main.commit=${COMMIT}"
LDFLAGS="${LDFLAGS} -X main.buildTime=${BUILD_TIME}"

SKIP_FRONTEND="${SKIP_FRONTEND:-false}"

if [ "${SKIP_FRONTEND}" != "true" ] && [ -f frontend/package.json ]; then
    echo "Building frontend..."
    cd frontend && yarn build && cd ..

    echo "Copying frontend dist to embed location..."
    rm -rf cmd/go-mumble-server/frontend-dist
    cp -r frontend/dist cmd/go-mumble-server/frontend-dist
fi

echo "Building go-mumble-server ${VERSION} (${COMMIT})..."

CGO_ENABLED=1 go build -ldflags "${LDFLAGS}" -o build/go-mumble-server ./cmd/go-mumble-server

echo "Done. Binary at build/go-mumble-server"
