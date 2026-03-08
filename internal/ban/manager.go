package ban

import "net"

// Manager maintains the server ban list.
type Manager struct{}

// IsBanned returns true if the address or certificate is banned.
func (m *Manager) IsBanned(addr net.Addr, certHash string) bool {
	// TODO: implement
	return false
}
