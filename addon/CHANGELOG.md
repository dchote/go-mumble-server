# Changelog

## 0.1.4

- Fix UserState proto2 field presence so Mumla/Plumble can unmute after self-mute
- Align mute/deaf cascade and Suppress sync with murmur; harden voice-path locking

## 0.1.3

- Set Home Assistant add-on maintainer and image author/url metadata to Daniel Chote

## 0.1.2

- Dependency and toolchain maintenance (Go modules, frontend packages, Go 1.25 alignment)

## 0.1.1

- Fix "Open Web UI" under HA ingress: do not inject `<base href="/">` when request path is `/` so asset URLs resolve under the ingress path

## 0.1.0

- Initial Home Assistant add-on release
- Mumble protocol on configurable port (default 64738)
- REST API and web UI via ingress ("Open Web UI")
- Configuration via add-on options (ports, limits, TLS, Bonjour, etc.)
- Persistent data in `/data`
