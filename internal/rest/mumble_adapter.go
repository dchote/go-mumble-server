package rest

import (
	"github.com/dchote/go-mumble-server/internal/user"
)

// MumbleUserAdapter adapts user.Manager to ConnectedUserLister.
type MumbleUserAdapter struct {
	*user.Manager
}

// ListConnected returns connected users for REST API.
func (a *MumbleUserAdapter) ListConnected() interface{} {
	users := a.Manager.ListAll()
	out := make([]ConnectedUser, len(users))
	for i, u := range users {
		out[i] = ConnectedUser{
			SessionID: u.SessionID,
			UserID:    u.UserID,
			Name:      u.Name,
			ChannelID: u.ChannelID,
		}
	}
	return out
}
