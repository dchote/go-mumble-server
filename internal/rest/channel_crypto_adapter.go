package rest

import (
	"github.com/dchote/go-mumble-server/internal/handler"
	"github.com/dchote/go-mumble-server/internal/mumble"
)

// MumbleChannelCryptoAdapter adapts mumble.Server to ChannelCryptoLister.
type MumbleChannelCryptoAdapter struct {
	Server   *mumble.Server
	ServerID uint
}

// ChannelCryptoModes implements handler.ChannelCryptoLister.
func (a *MumbleChannelCryptoAdapter) ChannelCryptoModes(serverID uint) map[uint32]string {
	if serverID != a.ServerID {
		return nil
	}
	return a.Server.AllChannelCryptoModes()
}

var _ handler.ChannelCryptoLister = (*MumbleChannelCryptoAdapter)(nil)
