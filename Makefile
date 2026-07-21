# go-mumble-server Makefile
# Requires: Docker, Node 20+ and yarn (for frontend), or use frontend-prebuilt

REPO_SLUG ?= dchote/go-mumble-server
GORELEASER_IMAGE ?= ghcr.io/goreleaser/goreleaser-cross:v1.25.9

.PHONY: frontend build-deb release-snapshot

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
