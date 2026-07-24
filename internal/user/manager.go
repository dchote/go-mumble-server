package user

import (
	"sync"

	"github.com/dchote/go-mumble-server/pkg/mumble"
	"gorm.io/gorm"
)

// SpeakGate is the minimum state the voice path needs to decide whether a
// session may send audio and which channel it occupies. Copied under the
// manager lock so the UDP goroutine never reads through an unprotected pointer.
type SpeakGate struct {
	mumble.VoiceState
	UserID    uint32
	ChannelID uint32
}

// Manager tracks connected Mumble user sessions.
type Manager struct {
	mu        sync.RWMutex
	bySession map[uint32]*mumble.User
	byName    map[string]*mumble.User
	pool      *sessionPool
	db        *gorm.DB
	maxUsers  int
}

// NewManager creates a UserManager.
func NewManager(db *gorm.DB, maxUsers int) *Manager {
	return &Manager{
		bySession: make(map[uint32]*mumble.User),
		byName:    make(map[string]*mumble.User),
		pool:      newSessionPool(maxUsers * 2),
		db:        db,
		maxUsers:  maxUsers,
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

// SetPing updates a user's ping (client-reported TCP RTT in ms). Thread-safe.
func (m *Manager) SetPing(sessionID uint32, ping float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u := m.bySession[sessionID]; u != nil {
		u.Ping = ping
	}
}

// Snapshot returns a deep copy of a user's record. Slice fields (texture, plugin
// context, access tokens) are cloned so later mutations of the live user cannot
// race with readers that hold the copy. Prefer this over GetUser for any read that
// outlives the manager lock.
func (m *Manager) Snapshot(sessionID uint32) (mumble.User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u := m.bySession[sessionID]
	if u == nil {
		return mumble.User{}, false
	}
	return cloneUser(u), true
}

// VoiceState returns a copy of a user's audio-routing flags. Used on the per-packet
// voice path, where copying the whole user record would be wasteful.
func (m *Manager) VoiceState(sessionID uint32) (mumble.VoiceState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u := m.bySession[sessionID]
	if u == nil {
		return mumble.VoiceState{}, false
	}
	return u.VoiceState, true
}

// SpeakGateFor returns the lean voice-path view of a session (flags + ACL keys).
func (m *Manager) SpeakGateFor(sessionID uint32) (SpeakGate, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u := m.bySession[sessionID]
	if u == nil {
		return SpeakGate{}, false
	}
	return SpeakGate{VoiceState: u.VoiceState, UserID: u.UserID, ChannelID: u.ChannelID}, true
}

// ChannelID returns a user's current channel under the read lock.
func (m *Manager) ChannelID(sessionID uint32) (uint32, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u := m.bySession[sessionID]
	if u == nil {
		return 0, false
	}
	return u.ChannelID, true
}

// UpdateUser applies fn to a user's record under the write lock and returns a deep
// copy of the resulting state, so callers can broadcast it without racing further
// edits of the live record.
func (m *Manager) UpdateUser(sessionID uint32, fn func(*mumble.User)) (mumble.User, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.bySession[sessionID]
	if u == nil {
		return mumble.User{}, false
	}
	fn(u)
	return cloneUser(u), true
}

func cloneUser(u *mumble.User) mumble.User {
	out := *u
	if u.Texture != nil {
		out.Texture = append([]byte(nil), u.Texture...)
	}
	if u.PluginContext != nil {
		out.PluginContext = append([]byte(nil), u.PluginContext...)
	}
	if u.AccessTokens != nil {
		out.AccessTokens = append([]string(nil), u.AccessTokens...)
	}
	return out
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
	id, hash, _, found := m.LookupAPIUser(username)
	return id, hash, found
}

// LookupAPIUser looks up an API user by username, returning id, password hash, role, and found.
// Used for RBAC-aware Mumble auth: id is users.id, role is needed for ACL resolution.
func (m *Manager) LookupAPIUser(username string) (id uint32, passwordHash string, role string, found bool) {
	var u struct {
		ID           uint
		PasswordHash string
		Role         string
	}
	err := m.db.Table("users").Where("username = ?", username).Select("id", "password_hash", "role").First(&u).Error
	if err != nil {
		return 0, "", "", false
	}
	return uint32(u.ID), u.PasswordHash, u.Role, true
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
