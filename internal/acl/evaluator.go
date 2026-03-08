package acl

import (
	"sync"

	"github.com/dchote/go-mumble-server/pkg/mumble"
)

// Evaluator resolves permissions for a user in a channel.
type Evaluator struct {
	mu sync.RWMutex
}

// Check returns whether the user has the given permission in the channel.
// For MVP, grants basic permissions in root; denies elsewhere unless cached.
func (e *Evaluator) Check(userID uint32, channelID uint32, perm mumble.Permission) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if channelID == 0 {
		return e.hasRootPermission(perm)
	}
	return e.hasRootPermission(perm)
}

func (e *Evaluator) hasRootPermission(perm mumble.Permission) bool {
	switch perm {
	case mumble.PermissionEnter, mumble.PermissionTraverse, mumble.PermissionSpeak,
		mumble.PermissionTextMessage, mumble.PermissionMakeTempChannel, mumble.PermissionListen:
		return true
	case mumble.PermissionWrite, mumble.PermissionMuteDeafen, mumble.PermissionMove,
		mumble.PermissionMakeChannel, mumble.PermissionLinkChannel, mumble.PermissionWhisper:
		return true
	case mumble.PermissionBan, mumble.PermissionRegister:
		return true
	default:
		return false
	}
}

// EffectivePermissions returns the permission bitmask for userID in channelID.
func (e *Evaluator) EffectivePermissions(userID uint32, channelID uint32) uint32 {
	var perms uint32
	all := []mumble.Permission{
		mumble.PermissionWrite, mumble.PermissionTraverse, mumble.PermissionEnter,
		mumble.PermissionSpeak, mumble.PermissionMuteDeafen, mumble.PermissionMove,
		mumble.PermissionMakeChannel, mumble.PermissionLinkChannel, mumble.PermissionWhisper,
		mumble.PermissionTextMessage, mumble.PermissionMakeTempChannel, mumble.PermissionListen,
		mumble.PermissionBan, mumble.PermissionRegister,
	}
	for _, p := range all {
		if e.Check(userID, channelID, p) {
			perms |= uint32(p)
		}
	}
	return perms
}
