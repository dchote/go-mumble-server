# Deploying go-mumble-server with Caddy

This guide covers deploying go-mumble-server on Debian with [Caddy](https://caddyserver.com/) as a reverse proxy for the REST API and web UI. Caddy provides automatic HTTPS (ACME/Let's Encrypt), TLS termination, and is the recommended proxy for production deployments.

## Requirements

- Debian 12 (Bookworm) or later
- Domain name pointed at your server (for automatic HTTPS)
- Ports 80 and 443 open (Caddy)
- Port 64738 open for Mumble TCP/UDP (or your configured Mumble port)

## Installing Caddy on Debian

Caddy is available in Debian's official repositories (Bookworm and later):

```bash
sudo apt update
sudo apt install caddy
```

Alternatively, for the latest Caddy release, add the official Caddy repository:

```bash
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install caddy
```

## Installing go-mumble-server

Install from a release package or build from source. See [README.md](../README.md#building) and [build-and-test.md](build-and-test.md) for build instructions.

If using the `.deb` package, go-mumble-server recommends Caddy (optional reverse proxy for the REST API):

```bash
sudo apt install ./go-mumble-server_*.deb
# When installing the .deb, apt will recommend Caddy if not already installed
```

## Configuring Caddy to Proxy the API

go-mumble-server serves the REST API and embedded web UI on port 64730 by default. Caddy runs in front to provide TLS and public access; restrict port 64730 with a firewall so only Caddy (on the same host) can reach it.

### 1. Ensure go-mumble-server is running

Use the default config or `/etc/go-mumble-server/mumble-server.toml`:

```toml
[network]
host = "0.0.0.0"
port = 64738
rest_port = 64730
```

go-mumble-server will listen on 0.0.0.0:64738 (Mumble) and 0.0.0.0:64730 (REST). Use a firewall (see below) to block 64730 from the internet.

### 2. Caddyfile for the REST API

Create `/etc/caddy/Caddyfile` (or your Caddy config path):

```
mumble.example.com {
    reverse_proxy 127.0.0.1:64730
}
```

Replace `mumble.example.com` with your domain. Caddy will:

- Obtain and renew TLS certificates automatically (Let's Encrypt)
- Serve the web UI and REST API at `https://mumble.example.com`
- Proxy `/api`, `/docs`, `/health`, and `/` to go-mumble-server

### 3. Optional: Subpath

If you want the API under a subpath (e.g. `https://example.com/mumble/`):

```
example.com {
    handle_path /mumble/* {
        reverse_proxy 127.0.0.1:64730
    }
    handle {
        respond "Not found" 404
    }
}
```

Note: The embedded frontend may need a base URL configuration for subpath deployments; the default build assumes root `/`.

### 4. Optional: Restrict to LAN

For local access only (no public domain):

```
:443 {
    tls internal
    reverse_proxy 127.0.0.1:64730
}
```

Or HTTP on a high port:

```
:8443 {
    reverse_proxy 127.0.0.1:64730
}
```

### 5. Reload Caddy

After editing the Caddyfile:

```bash
sudo systemctl reload caddy
```

## Firewall (recommended)

Allow Mumble (64738) and Caddy (80, 443); block direct access to the REST port (64730) from the internet:

```bash
# UFW example
sudo ufw allow 64738/tcp
sudo ufw allow 64738/udp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw deny 64730/tcp
sudo ufw enable
```

## Summary

| Service           | Port  | Bind    | Purpose                    |
|-------------------|-------|---------|----------------------------|
| go-mumble-server  | 64738 | 0.0.0.0 | Mumble protocol (TCP+UDP)  |
| go-mumble-server  | 64730 | 0.0.0.0 | REST API + web UI (firewall blocks from internet) |
| Caddy             | 80    | 0.0.0.0 | HTTP → HTTPS redirect      |
| Caddy             | 443   | 0.0.0.0 | HTTPS → reverse proxy      |

## See also

- [Caddy reverse_proxy directive](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy)
- [Caddy automatic HTTPS](https://caddyserver.com/docs/automatic-https)
- [Configuration](../README.md#configuration) — go-mumble-server config reference
