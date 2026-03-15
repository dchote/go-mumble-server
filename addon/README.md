# go-mumble-server Home Assistant Add-on

Mumble voice chat server (wire-compatible with Murmur). Use the **Open Web UI** button to manage virtual servers, channels, users, and ACLs. The REST API and embedded web UI run via Home Assistant ingress; the Mumble protocol port is exposed for client connections.

## Installation

1. In Home Assistant, go to **Settings** → **Add-ons** → **Add-on store**.
2. Click the three dots (⋮) → **Repositories**.
3. Add this repository URL: `https://github.com/dchote/go-mumble-server`
4. Find **go-mumble-server** in the add-on list and click **Install**.
5. Configure options if needed, then **Start** the add-on.

## Configuration

Options in the add-on configuration panel map to server settings:

- **Mumble port** — Port for Mumble protocol (default 64738). Must match the port exposed in the add-on **Network** section.
- **REST API port** — Internal port for the web UI and API (default 64730). Used by ingress; normally no need to change.
- **Max users / Max bandwidth** — Per–virtual-server limits.
- **Log level** — `debug`, `info`, `warn`, or `error`.
- **Welcome text** — Optional HTML message shown to connecting clients.
- **Server password** — Optional password required to connect.
- **SSL certificate / key** — Optional paths to TLS PEM files (e.g. from Home Assistant SSL). Leave empty for auto-generated self-signed certificates.
- **Bonjour / Register name** — LAN discovery (mDNS) settings.
- **Channel nesting limit / count limit** — Channel tree limits.
- **Certificate required** — Require client certificates for user authentication.

Data (database, TLS certs) is stored in the add-on’s persistent `/data` directory.

## Connecting with a Mumble client

1. Install a Mumble client (e.g. [Mumble](https://www.mumble.info/), [Plumble](https://play.google.com/store/apps/details?id=com.morlunk.mumbleclient.free) on Android).
2. Add a server: use your Home Assistant hostname or IP.
3. Set the port to **64738** (or the value you set for **Mumble port** in the add-on config).
4. If you set a **Server password**, enter it in the client.

The server uses self-signed TLS by default; clients will prompt to accept the certificate.

## Web UI (ingress)

Click **Open Web UI** in the add-on panel to open the management interface. You can:

- Create and manage virtual servers
- Edit channel trees and ACLs
- Register users and manage permissions
- View connected users and server status

The web UI and REST API are served through Home Assistant ingress (no separate port or firewall rules).

## Publishing (maintainers)

For the add-on to install, the container images must exist and be **public**:

1. **Build and push images** — In the repo go to **Actions** → **Addon** → **Run workflow**. This pushes `ghcr.io/owner/amd64-addon-go-mumble-server:0.1.0` and `ghcr.io/owner/aarch64-addon-go-mumble-server:0.1.0` (tag = addon `version` in `config.yaml`). Pushing a tag `v*` (e.g. `v0.1.0`) also runs the workflow and tags the image with the version without `v` (e.g. `0.1.0`).
2. **Make the package public** — On GitHub open the repo → **Packages** (right-hand side), or go to [ghcr.io](https://ghcr.io) and open the `addon-go-mumble-server` package. In **Package settings** set **Visibility** to **Public** so the Home Assistant Supervisor can pull without authentication. Do this for both `amd64-addon-go-mumble-server` and `aarch64-addon-go-mumble-server` if they appear separately.

If install fails with **404** or **manifest unknown**, the image tag likely doesn’t match the addon version (Supervisor uses `config.yaml` version as the image tag) or the package is still private.

## Forks

If you install the add-on from a fork of this repository, the add-on will still try to pull images from the upstream image URL (`ghcr.io/dchote/...`). To use your own images, either:

- Build and push images from your fork (e.g. via the addon CI workflow), then edit the add-on’s `config.yaml` in your repo to set `image` to your registry and image name (e.g. `ghcr.io/your-username/{arch}-addon-go-mumble-server`), or  
- Use the upstream repository URL in Home Assistant so the add-on uses the official images.

## Support

- [GitHub repository](https://github.com/dchote/go-mumble-server)
- [Project documentation](https://github.com/dchote/go-mumble-server#readme)
