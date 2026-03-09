# 0001: API & Frontend editing of servers, channels, users, groups, ACLs, and config migration

## Status: Implemented

## Summary

- Virtual server CRUD (create, read, update, delete) via REST API and Vuetify UI
- Channel CRUD within virtual servers
- Config migration from TOML to SQLite (meta_config, server_configs)
- Bans API and UI
- ACL and groups persistence and API (editable UI), full Mumble-compatible evaluator
- Registered Mumble users API and UI
- Server configuration pages (per-server and global) following recipe-project pattern

## Implementation

### Phase 1: Config migration to SQLite
- `meta_config` table for process-level settings
- Extended `server_configs` with channel limits, default_channel, cert_required
- `internal/config/dbconfig.go` for loading/updating DB config
- Server startup loads meta + server config from DB; TOML used for bootstrap (DB path, TLS paths, log level)

### Phase 2: Virtual server CRUD
- REST: POST/GET/PATCH/DELETE /api/v1/servers
- Frontend: CreateServerDialog, EditServerDialog, delete confirmation
- Servers list page with toolbar Create button and per-card actions

### Phase 3: Channel CRUD
- REST: POST/PATCH/DELETE /api/v1/servers/{id}/channels, channels/{channelId}
- Channel manager wired to REST via GetChannelManager
- Frontend: CreateChannelDialog, EditChannelDialog, context menu on channel nodes

### Phase 4: ACL and groups
- Protocol correctness: Channel ID 0 = root channel (required by Mumble). `ChannelState` always emits `channel_id` (including 0); root omits `parent` (proto2 optional). `UserState` always emits `session` and `channel_id` (including users in root). Handlers (ACL, VoiceTarget, PermissionQuery) treat 0 as root. The channel manager auto-migrates pre-existing databases where root has the wrong ID.
- DB models: `channel_groups`, `channel_acls` (with `eval_here`, `invert` for selector modifiers)
- REST: GET/PUT /api/v1/servers/{id}/channels/{channelId}/acl
- ACLDialog (editable) in channel context menu — add/remove groups and ACL rules, permission checkboxes
- Full ACL evaluator: `internal/acl/evaluator.go` — DB-backed, in-memory cache, meta groups (@all, @in, @out, @sub, @auth, @admin), token groups, eval-locality (~), inversion (!)
- Default root ACLs seeded on server/channel creation: `all` (Traverse, Enter, Speak, Whisper, TextMessage, Listen), `auth` (MakeTempChannel, SelfRegister), `admin` (Write)
- UserID from registered_users or API users; unregistered users get 0; API users get synthetic userIDs and RBAC roles for @admin

### Phase 5: Bans
- REST: GET/POST/DELETE /api/v1/servers/{id}/bans, bans/{banId}
- Bans section on server detail with Add ban dialog

### Phase 6a: API user Mumble auth (RBAC)
- Management API users can authenticate to Mumble with their web credentials (even when no server password)
- API admins (role=admin) receive @admin privileges on all virtual servers
- Connected users REST response includes `is_admin` for channel tree display

### Phase 6: Registered Mumble users
- DB model: `registered_users`
- REST: GET/POST/PATCH/DELETE /api/v1/servers/{id}/registered-users
- Registered users section on server detail

### Phase 7: Server config editor
- REST: GET/PATCH /api/v1/servers/{id}/config, GET/PATCH /api/v1/meta/config
- pages/admin/settings.vue (meta config), pages/servers/[id]/config.vue (per-server)
- Settings nav link, Configuration link on server detail

## TLS and bootstrap

- TOML retains: database.path, tls.cert, tls.key, logging.level, frontend_embed
- All other settings stored in SQLite
