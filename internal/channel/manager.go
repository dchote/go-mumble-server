package channel

import "github.com/dchote/go-mumble-server/pkg/mumble"

// Manager maintains the channel tree state.
type Manager struct{}

// GetChannel returns a channel by ID.
func (m *Manager) GetChannel(id uint32) (*mumble.Channel, bool) {
	// TODO: implement
	return nil, false
}
