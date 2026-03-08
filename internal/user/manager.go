package user

import (
	"sync"

	"github.com/dchote/go-mumble-server/pkg/mumble"
	"gorm.io/gorm"
)

// Manager tracks connected Mumble user sessions.
type Manager struct {
	mu       sync.RWMutex
	bySession map[uint32]*mumble.User
	byName   map[string]*mumble.User
	pool     *sessionPool
	db       *gorm.DB
	maxUsers int
}

// NewManager creates a UserManager.
func NewManager(db *gorm.DB, maxUsers int) *Manager {
	return &Manager{
		bySession: make(map[uint32]*mumble.User),
		byName:   make(map[string]*mumble.User),
		pool:     newSessionPool(maxUsers * 2),
		db:       db,
		maxUsers: maxUsers,
	}
}

// Add adds a connected user. Returns false if username in use or server full.
func (m *Manager) Add(u *mumble.User) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byName[u.Name] != nil {
		return false
	}
	if len(m.bySession) >= m.maxUsers && m.maxUsers > 0 {
		return false
	}
	sid, ok := m.pool.Alloc()
	if !ok {
		return false
	}
	u.SessionID = sid
	m.bySession[sid] = u
	m.byName[u.Name] = u
	return true
}

// Remove removes a user by session ID.
func (m *Manager) Remove(sessionID uint32) *mumble.User {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.bySession[sessionID]
	if u == nil {
		return nil
	}
	delete(m.bySession, sessionID)
	delete(m.byName, u.Name)
	m.pool.Free(sessionID)
	return u
}

// GetUser returns a user by session ID.
func (m *Manager) GetUser(sessionID uint32) (*mumble.User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.bySession[sessionID]
	return u, ok
}

// GetByName returns a user by username.
func (m *Manager) GetByName(name string) (*mumble.User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.byName[name]
	return u, ok
}

// SessionIDsInChannel returns session IDs of users in the given channel.
func (m *Manager) SessionIDsInChannel(channelID uint32) []uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []uint32
	for _, u := range m.bySession {
		if u.ChannelID == channelID {
			out = append(out, u.SessionID)
		}
	}
	return out
}

// ListByChannel returns users in the given channel.
func (m *Manager) ListByChannel(channelID uint32) []*mumble.User {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*mumble.User
	for _, u := range m.bySession {
		if u.ChannelID == channelID {
			out = append(out, u)
		}
	}
	return out
}

// ListAll returns all connected users.
func (m *Manager) ListAll() []*mumble.User {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*mumble.User, 0, len(m.bySession))
	for _, u := range m.bySession {
		out = append(out, u)
	}
	return out
}

// Count returns the number of connected users.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.bySession)
}

// SetChannel updates a user's channel.
func (m *Manager) SetChannel(sessionID uint32, channelID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u := m.bySession[sessionID]; u != nil {
		u.ChannelID = channelID
	}
}

// RegisterDBUser looks up a management/registered user by username.
func (m *Manager) RegisterDBUser(username string) (userID uint32, passwordHash string, found bool) {
	var u struct {
		ID           uint
		PasswordHash string
	}
	err := m.db.Table("users").Where("username = ?", username).Select("id", "password_hash").First(&u).Error
	if err != nil {
		return 0, "", false
	}
	return uint32(u.ID), u.PasswordHash, true
}

// sessionPool allocates session IDs.
type sessionPool struct {
	mu    sync.Mutex
	next  uint32
	free  []uint32
	inUse map[uint32]bool
	max   int
}

func newSessionPool(max int) *sessionPool {
	return &sessionPool{
		next:  1,
		free:  nil,
		inUse: make(map[uint32]bool),
		max:   max,
	}
}

func (p *sessionPool) Alloc() (uint32, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.inUse) >= p.max && p.max > 0 {
		return 0, false
	}
	if len(p.free) > 0 {
		sid := p.free[len(p.free)-1]
		p.free = p.free[:len(p.free)-1]
		p.inUse[sid] = true
		return sid, true
	}
	sid := p.next
	p.next++
	if p.max > 0 && sid >= uint32(p.max*2) {
		return 0, false
	}
	p.inUse[sid] = true
	return sid, true
}

func (p *sessionPool) Free(sessionID uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.inUse[sessionID] {
		return
	}
	delete(p.inUse, sessionID)
	p.free = append(p.free, sessionID)
}
