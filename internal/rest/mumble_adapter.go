package rest

import (
	"github.com/dchote/go-mumble-server/internal/acl"
	"github.com/dchote/go-mumble-server/internal/mumble"
	"github.com/dchote/go-mumble-server/internal/user"
	"gorm.io/gorm"
)

// MumbleUserAdapter adapts user.Manager to ConnectedUserLister.
type MumbleUserAdapter struct {
	*user.Manager
	Server *mumble.Server // optional: used for voice transport (udp/tcp)
}

// ListConnected returns connected users for REST API with is_admin resolved for display.
func (a *MumbleUserAdapter) ListConnected(serverID uint, db *gorm.DB, includeSensitive bool) interface{} {
	users := a.Manager.SnapshotAll()
	out := make([]ConnectedUser, len(users))
	for i, u := range users {
		isAdmin := db != nil && acl.IsAdminForDisplay(db, serverID, u.UserID)
		address := ""
		certHash := ""
		if includeSensitive {
			address = u.Address
			certHash = u.CertHash
		}
		voiceTransport := "tcp"
		if a.Server != nil && a.Server.HasUDPAddress(u.SessionID) {
			voiceTransport = "udp"
		}
		out[i] = ConnectedUser{
			SessionID:       u.SessionID,
			UserID:          u.UserID,
			Name:            u.Name,
			ChannelID:       u.ChannelID,
			Address:         address,
			Ping:            u.Ping,
			CertificateHash: certHash,
			CryptoMode:      u.CryptoMode,
			VoiceTransport:  voiceTransport,
			SelfMute:        u.SelfMute,
			SelfDeaf:        u.SelfDeaf,
			Mute:            u.Mute,
			Deaf:            u.Deaf,
			IsAdmin:         isAdmin,
		}
	}
	return out
}
