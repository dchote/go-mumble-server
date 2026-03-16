# Changelog

## 0.1.1

- Fix "Open Web UI" under HA ingress: do not inject `<base href="/">` when request path is `/` so asset URLs resolve under the ingress path

## 0.1.0

- Initial Home Assistant add-on release
- Mumble protocol on configurable port (default 64738)
- REST API and web UI via ingress ("Open Web UI")
- Configuration via add-on options (ports, limits, TLS, Bonjour, etc.)
- Persistent data in `/data`
