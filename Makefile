# go-mumble-server Makefile
# Requires: Docker, Node 20+ and yarn (for frontend), or use frontend-prebuilt

REPO_SLUG ?= dchote/go-mumble-server
GORELEASER_IMAGE ?= ghcr.io/goreleaser/goreleaser-cross:v1.25.9

.PHONY: frontend build-deb release-snapshot fmt fmt-check vet test check

# Go sources, excluding the vendored reference implementations under research/ and
# the Go files that ship inside frontend dependencies. Walking the tree rather than
# asking git keeps new files in the gate and keeps deleted ones out of it.
GO_FILES = $(shell find . -name '*.go' -not -path './research/*' -not -path './frontend/node_modules/*')

# Packages under this module that belong to the server. ./... would also pick up
# accidental Go sources under frontend/node_modules (e.g. flatted's golang pkg).
GO_PKGS = ./cmd/... ./internal/... ./pkg/...

fmt:
	gofmt -w $(GO_FILES)

fmt-check:
	@unformatted=$$(gofmt -l $(GO_FILES)); \
	if [ -n "$$unformatted" ]; then echo "gofmt needed for:"; echo "$$unformatted"; exit 1; fi

vet:
	go vet $(GO_PKGS)

test:
	CGO_ENABLED=1 go test -race $(GO_PKGS)

# What CI runs, minus the frontend build.
check: fmt-check vet test

# Build frontend and copy to embed location (required before build-deb / release-snapshot).
frontend:
	cd frontend && yarn install --frozen-lockfile && yarn build
	rm -rf cmd/go-mumble-server/frontend-dist
	cp -r frontend/dist cmd/go-mumble-server/frontend-dist

# Build .deb packages locally via Docker (same as CI). Works on macOS and Linux.
# Produces dist/*.deb (amd64 and arm64). Requires Docker.
# Uses scripts/build-deb.sh (two runs: amd64 and arm64 in matching containers, then merge).
build-deb: frontend
	SKIP_FRONTEND=1 ./scripts/build-deb.sh

# Alias for build-deb (same behavior).
release-snapshot: build-deb
