# 0006: Dependency Update Maintenance Release

## Status: Implemented

## Summary

Maintenance pass: bring Go modules and frontend packages current, align the Go 1.25 toolchain across CI/Docker/goreleaser, run lint/test/build, then ship **v0.1.2** so upstream fixes land in GitHub Release artifacts and Home Assistant add-on images.

## Goal

- Update all Go and frontend dependencies
- Align toolchain (Go 1.25) across CI, Dockerfiles, and goreleaser-cross
- Fix broken frontend `yarn lint` (ESLint was referenced but not installed)
- Verify with lint + test + embed build
- Cut patch release **v0.1.2** (binaries/debs + HA add-on images)

## Changes

### Go modules

- `go get -u ./...` then `go mod tidy`
- Minimal code fixes if upgrades break APIs

### Frontend

- Upgrade Vue 3 / Vuetify 3 / Vite 6 line packages; regenerate `yarn.lock`
- Add ESLint + `eslint-plugin-vue` and flat config so `yarn lint` works
- Confirm `yarn build` still produces embeddable dist

### Toolchain alignment (Go 1.25)

| Location | Change |
|----------|--------|
| `.github/workflows/ci.yml` | `go-version: "1.25"` |
| `Dockerfile` | `golang:1.25-bookworm` |
| `.github/workflows/release.yml` | goreleaser-cross `v1.25.x` |
| `Makefile` / `scripts/build-deb.sh` | same goreleaser-cross image |
| `docs/build-and-test.md` | Document Go 1.25 for CI/Docker/release |

### Version bump (v0.1.2)

- `addon/config.yaml`, `addon/CHANGELOG.md`
- `frontend/package.json`, `api/openapi.yaml`

### Verification

1. `cd frontend && yarn install && yarn lint && yarn build`
2. Copy dist → `cmd/go-mumble-server/frontend-dist`
3. `go vet ./...`
4. `CGO_ENABLED=1 go test -race -timeout=60s ./...`
5. `CGO_ENABLED=1 go build -tags embed_frontend -o /dev/null ./cmd/go-mumble-server`

### Release

- Commit, tag `v0.1.2`, push — triggers release + addon workflows

## Out of scope

- Feature work, protocol changes, Dependabot/Renovate, major framework migrations unless forced by peers
