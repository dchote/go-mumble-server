# 0004: Voice Path Debugging

## Status: Implemented

## Summary

Configurable voice path debug logging to diagnose UDP/TCP tunnel and routing issues (e.g. ESP32-mumble native UDP transport bugs). The feature exposes a per-server `voice_debug` setting via REST API and frontend, with runtime toggle support.

## Problem

The codebase had ad-hoc `[VOICE-DEBUG]` logging always enabled in the router, SendAudio, and handleUDPTunnel paths. This helped diagnose issues but produced verbose logs in production. There were no debug points in HandleUDP (session identification, decrypt, ping echo/suppress) to trace native UDP voice transport problems.

## Changes

### Data model and config

- Added `voice_debug` (bool, default false) to `server_configs` table via `models.ServerConfig`
- `ServerConfigData`, `LoadServerConfig`, `ConfigForServer`, `EnsureServerConfig` updated
- `Config.VoiceDebug` wired through for Mumble protocol use

### Mumble server integration

- `Server.voiceDebug` (atomic.Bool) and `SetVoiceDebug(bool)` for runtime updates
- All `[VOICE-DEBUG]` logs gated by the flag in router, SendAudio, handleUDPTunnel
- `RouterConfig.VoiceDebug` and `Router.SetVoiceDebug()` for runtime sync

### REST API

- `GET /api/v1/servers/:id/config` returns `voice_debug`
- `PATCH /api/v1/servers/:id/config` accepts `voice_debug`
- `OnConfigChange` callback invokes `ms.SetVoiceDebug()` when config is updated so toggling takes effect without restart

### Frontend

- Server config page (`pages/servers/[id]/config.vue`) adds a checkbox: "Enable voice path debugging (verbose logs for UDP/TCP tunnel diagnosis)"

### HandleUDP debug points

- Session identification: log cache hit vs trial-decrypt when voice_debug enabled
- Decrypt failure: log when trial-decrypt fails for an address (no matching session). If another port from the same host is already mapped (e.g. Mumble client's probe port vs voice port), the message is down-leveled to Debug to reduce log noise.
- Ping echo vs suppress: log when ping is echoed or suppressed (mixed-crypto channel)

### OpenAPI

- Added `ServerConfig` schema and `voice_debug` to GET response and PATCH body for `servers/{id}/config`

## Files modified

- `internal/database/models/server_config.go` — VoiceDebug column
- `internal/config/dbconfig.go` — ServerConfigData, LoadServerConfig, ConfigForServer, EnsureServerConfig
- `internal/config/config.go` — Config.VoiceDebug
- `internal/audio/router.go` — RouterConfig.VoiceDebug, SetVoiceDebug, gate logs
- `internal/mumble/handlers.go` — voiceDebug field, SetVoiceDebug, gate logs, HandleUDP debug points
- `internal/handler/server.go` — GetConfig/UpdateConfig voice_debug, OnConfigChange
- `internal/rest/router.go` — OnConfigChange param
- `internal/server/server.go` — onConfigChange implementation
- `internal/rest/router_security_test.go` — extra nil for RouterWithMumble
- `frontend/src/pages/servers/[id]/config.vue` — voice_debug checkbox
- `api/openapi.yaml` — ServerConfig schema, servers/{id}/config endpoints, voice_debug

## Compatibility

- GORM AutoMigrate adds `voice_debug` column to existing databases (default false)
- REST API: additive (new optional field)
- Frontend: additive checkbox
