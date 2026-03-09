package rest

import (
	"github.com/dchote/go-mumble-server/internal/acl"
	"github.com/dchote/go-mumble-server/internal/user"
	"gorm.io/gorm"
)

// MumbleUserAdapter adapts user.Manager to ConnectedUserLister.
type MumbleUserAdapter struct {
	*user.Manager
}

// ListConnected returns connected users for REST API with is_admin resolved for display.
func (a *MumbleUserAdapter) ListConnected(serverID uint, db *gorm.DB) interface{} {
	users := a.Manager.ListAll()
	out := make([]ConnectedUser, len(users))
	for i, u := range users {
		isAdmin := db != nil && acl.IsAdminForDisplay(db, serverID, u.UserID)
		out[i] = ConnectedUser{
			SessionID:       u.SessionID,
			UserID:          u.UserID,
			Name:            u.Name,
			ChannelID:       u.ChannelID,
			Address:         u.Address,
			Ping:            u.Ping,
			CertificateHash: u.CertHash,
			SelfMute:        u.SelfMute,
			SelfDeaf:        u.SelfDeaf,
			Mute:            u.Mute,
			Deaf:            u.Deaf,
			IsAdmin:         isAdmin,
		}
	}
	return out
}
