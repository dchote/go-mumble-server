package rest

import (
	"github.com/dchote/go-mumble-server/internal/handler"
	"github.com/dchote/go-mumble-server/internal/mumble"
)

// MumbleUserActionAdapter adapts mumble.Server to ConnectedUserActioner.
// Only supports the single virtual server ID it was constructed with.
type MumbleUserActionAdapter struct {
	Server   *mumble.Server
	ServerID uint
}

// Kick implements handler.ConnectedUserActioner.
func (a *MumbleUserActionAdapter) Kick(serverID uint, sessionID uint32, reason string) bool {
	if serverID != a.ServerID {
		return false
	}
	return a.Server.KickSession(sessionID, reason)
}

// Mute implements handler.ConnectedUserActioner.
func (a *MumbleUserActionAdapter) Mute(serverID uint, sessionID uint32, mute bool) bool {
	if serverID != a.ServerID {
		return false
	}
	return a.Server.MuteSession(sessionID, mute)
}

// BanAndKick implements handler.ConnectedUserActioner.
func (a *MumbleUserActionAdapter) BanAndKick(serverID uint, sessionID uint32, reason string) bool {
	if serverID != a.ServerID {
		return false
	}
	return a.Server.BanAndKickSession(sessionID, reason)
}

// Ensure we implement the interface.
var _ handler.ConnectedUserActioner = (*MumbleUserActionAdapter)(nil)
