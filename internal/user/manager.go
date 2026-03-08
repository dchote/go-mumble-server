package user

import "github.com/dchote/go-mumble-server/pkg/mumble"

// Manager tracks connected user sessions.
type Manager struct{}

// GetUser returns a user by session ID.
func (m *Manager) GetUser(sessionID uint32) (*mumble.User, bool) {
	// TODO: implement
	return nil, false
}
