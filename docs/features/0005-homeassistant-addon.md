# 0005: Home Assistant Add-on

## Status: Implemented

## Summary

The repository doubles as a Home Assistant add-on repository. Users can add `https://github.com/dchote/go-mumble-server` in Home Assistant and install the **go-mumble-server** add-on. The add-on exposes server configuration via HA options, the REST API and web UI via ingress ("Open Web UI"), and the Mumble protocol port for client connections.

## Architecture

- **Repository layout**: Addon files live in the `addon/` subdirectory (slug in `config.yaml` is `go-mumble-server`; the directory is named `addon` to avoid conflicting with the `go-mumble-server` binary at repo root). `repository.yaml` at repo root declares the add-on repository.
- **Pre-built images**: CI builds multi-arch Docker images from repo root context (`addon/Dockerfile`) and pushes to GHCR. The add-on `config.yaml` references these images so the Supervisor pulls them instead of building locally.
- **Networking**: Ingress serves the web UI and REST API on port 64730. Ports 64738/tcp and 64738/udp are exposed for Mumble client connections.
- **Configuration**: The s6 `run` script in `addon/rootfs/` reads HA addon options via `bashio::config` and sets `MUMBLE_*` environment variables before exec’ing the server binary. The server already supported env-based config; additional env vars were added for welcome text, server password, channel limits, and cert required.

## Changes

### New files

- `repository.yaml` — Add-on repository metadata (name, url, maintainer).
- `addon/config.yaml` — Add-on metadata, options schema, ingress, ports, image reference.
- `addon/build.yaml` — Debian base images per arch (hassio-addons), OCI labels.
- `addon/Dockerfile` — Multi-stage build (Node frontend → Go binary → HA base); expects build context at repo root.
- `addon/rootfs/etc/services.d/go-mumble-server/run` — s6 run script: sets `MUMBLE_*` from options, execs binary.
- `addon/rootfs/etc/services.d/go-mumble-server/finish` — s6 finish script for exit handling.
- `addon/translations/en.yaml` — Option names and descriptions for the HA config UI.
- `addon/README.md` — Add-on user documentation.
- `addon/CHANGELOG.md` — Add-on changelog.
- `.github/workflows/addon.yml` — Build and push amd64/aarch64 images to GHCR on tags, main (addon-related paths), and workflow_dispatch.

### Modified files

- `internal/config/config.go` — Added env support for `MUMBLE_WELCOME_TEXT`, `MUMBLE_SERVER_PASSWORD`, `MUMBLE_CHANNEL_NESTING_LIMIT`, `MUMBLE_CHANNEL_COUNT_LIMIT`, `MUMBLE_DEFAULT_CHANNEL`, `MUMBLE_CERT_REQUIRED`.
- `frontend/src/utils/ingress.js` — Shared `getIngressBase()` for the HA ingress path prefix; used by api and router so requests and routes stay under the ingress path.
- `frontend/src/utils/api.js` — API base derived via `getIngressBase()` (e.g. `/api/hassio_ingress/<token>/api/v1` when under ingress).
- `frontend/src/router/index.js` — Router base derived via `getIngressBase()` so routes resolve under the ingress path.
- `frontend/vite.config.js` — `base: './'` for relative asset paths; Go server injects `<base href="...">` when serving index.html so assets load correctly on direct navigation or reload.
- `Dockerfile` (root) — Added `-tags embed_frontend` to `go build` so the standalone image embeds the frontend.
- `README.md` — Added "Home Assistant Add-on" section with repo URL and link to addon docs.

### Addon options (config.yaml)

Options map to server env vars: `mumble_port`, `rest_port`, `max_users`, `max_bandwidth`, `log_level`, `welcome_text`, `server_password`, `ssl_cert`, `ssl_key`, `bonjour`, `register_name`, `channel_nesting_limit`, `channel_count_limit`, `cert_required`. Database path is fixed to `/data/mumble-server.sqlite` (addon persistent storage).

## Notes

- **Icon/logo**: Add-on `icon.png` and `logo.png` can be added later; HA will fall back to a default if missing.
- **Image naming**: Config uses `image: "ghcr.io/dchote/{arch}-addon-go-mumble-server"`. CI pushes to `ghcr.io/<repository_owner>/<arch>-addon-go-mumble-server:<tag>` (tag = version tag or `latest`).
- **Frontend under ingress**: The Vue app and API use path-aware base URLs via `getIngressBase()` so they work when served under `/api/hassio_ingress/<token>/`. The Go server injects a `<base href>` when serving index.html so relative asset URLs resolve correctly on reload or direct links.
