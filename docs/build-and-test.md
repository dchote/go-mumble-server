# Build and Test Guide

## Prerequisites

- **Go 1.24+** (1.25 used in development)
- **CGO enabled** — required for SQLite (GORM driver)
- **Node.js 20+ and Yarn** — only for full builds with embedded frontend

## Building

### Server Only (no frontend)

Fastest path. Skips Vue build, uses stub that returns `nil` for embedded frontend:

```bash
SKIP_FRONTEND=true ./scripts/build.sh
# Binary at build/go-mumble-server
```

Or directly:

```bash
CGO_ENABLED=1 go build -o go-mumble-server ./cmd/go-mumble-server
```

### Full Build (with embedded frontend)

Requires Node.js/Yarn. Builds Vue, copies to embed location, compiles with `embed_frontend` tag:

```bash
./scripts/build.sh
# Binary at build/go-mumble-server
```

**Important:** The `embed_frontend` build tag requires `cmd/go-mumble-server/frontend-dist/` to exist with at least one file. Without it:

```
pattern frontend-dist: cannot embed directory frontend-dist: contains no embeddable files
```

Always run `./scripts/build.sh` (without `SKIP_FRONTEND=true`) or build the frontend first:

```bash
cd frontend && yarn build && cd ..
cp -r frontend/dist cmd/go-mumble-server/frontend-dist
CGO_ENABLED=1 go build -tags embed_frontend -o build/go-mumble-server ./cmd/go-mumble-server
```

### Building Debian packages (.deb) on macOS or Linux

To build `.deb` packages locally (same as CI), use the Makefile or the build script. Both use Docker and [goreleaser-cross](https://github.com/goreleaser/goreleaser-cross) so you get the same artifacts as the release workflow. The script works on Linux and macOS. It runs two containers (one `linux/amd64`, one `linux/arm64`) so CGO uses the correct toolchain for each arch, then merges the outputs into `dist/`. The arch-specific configs `.goreleaser-amd64.yaml` and `.goreleaser-arm64.yaml` are used for these runs.

**Requirements:** Docker, Node 20+ and Yarn (for the frontend).

```bash
make build-deb
# or
./scripts/build-deb.sh
```

To skip the frontend build (e.g. you already ran `make frontend`): `SKIP_FRONTEND=1 ./scripts/build-deb.sh`

This builds the frontend, then runs GoReleaser in snapshot mode inside Docker. Output is under **`dist/`**:

- `dist/*_amd64.deb` — Debian package for Linux amd64
- `dist/*_arm64.deb` — Debian package for Linux arm64

To only build the frontend (e.g. before running the Docker step yourself): `make frontend`.

## Releases

GitHub Releases are produced when you push a version tag (e.g. `v1.0.0`). Each release includes:

- **Linux binaries** — amd64 and arm64
- **Debian packages** — `*_amd64.deb` and `*_arm64.deb` for easy installation on Debian/Ubuntu

The release workflow (`.github/workflows/release.yml`) runs on tag push and publishes these artifacts to the GitHub Release. You can also run the workflow manually via **Actions → Release → Run workflow** to build a snapshot (no publish); the `.deb` files are then available as workflow artifacts.

## Running Unit Tests

### Basic run

```bash
CGO_ENABLED=1 go test ./...
```

### With timeout (avoids hangs)

If tests block (e.g. missing mocks, network calls), use a timeout:

```bash
CGO_ENABLED=1 go test -timeout=30s ./...
```

### With race detector

```bash
CGO_ENABLED=1 go test -race -timeout=60s ./...
```

Race detection slows tests; 60s timeout is recommended.

### Verbose output

```bash
CGO_ENABLED=1 go test -v -timeout=30s ./...
```

### Test a single package

```bash
CGO_ENABLED=1 go test -timeout=30s ./pkg/mumble/protocol/...
```

## Vet and Lint

```bash
go vet ./...
```

## CI Alignment

The `.github/workflows/ci.yml` workflow runs:

1. `go mod download`
2. `go vet ./...`
3. `CGO_ENABLED=1 go test -race -v ./...`
4. Frontend build + `go build -tags embed_frontend`

Use the same commands locally to catch CI failures.

## Common Issues

### Tests hang

- Add `-timeout=30s` (or higher) to `go test`
- Check for tests that start servers, open sockets, or block on I/O without mocks
- Use `go test -run TestName` to isolate a specific test

### Build fails: "cannot embed directory frontend-dist"

- Run full build: `./scripts/build.sh` (no `SKIP_FRONTEND`)
- Or use server-only: `SKIP_FRONTEND=true ./scripts/build.sh`

### CGO errors (SQLite)

- Ensure CGO is enabled: `CGO_ENABLED=1`
- On Linux CI, install `libsqlite3-dev` before building
- On macOS, Xcode Command Line Tools usually provide what's needed

### go.mod version mismatch

- `go.mod` may specify Go 1.25; CI may use 1.24
- Use `go version` to confirm local Go; tests and build work with 1.24+
