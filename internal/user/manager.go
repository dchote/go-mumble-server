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

// Add takes ownership of a connected user and returns the stored record with its
// allocated session ID. Returns false if the username is in use or the server is
// full. The manager copies the record, so the caller's value never aliases the
// live one.
func (m *Manager) Add(u mumble.User) (mumble.User, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byName[u.Name] != nil {
		return mumble.User{}, false
	}
	if len(m.bySession) >= m.maxUsers && m.maxUsers > 0 {
		return mumble.User{}, false
	}
	sid, ok := m.pool.Alloc()
	if !ok {
		return mumble.User{}, false
	}
	stored := cloneUser(&u)
	stored.SessionID = sid
	m.bySession[sid] = &stored
	m.byName[stored.Name] = &stored
	return cloneUser(&stored), true
}

// Remove removes a user by session ID, returning a copy of the removed record.
func (m *Manager) Remove(sessionID uint32) (mumble.User, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.bySession[sessionID]
	if u == nil {
		return mumble.User{}, false
	}
	delete(m.bySession, sessionID)
	delete(m.byName, u.Name)
	m.pool.Free(sessionID)
	return cloneUser(u), true
}

// Exists reports whether a session is connected.
func (m *Manager) Exists(sessionID uint32) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bySession[sessionID] != nil
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

// CountInChannel returns how many users occupy the given channel.
func (m *Manager) CountInChannel(channelID uint32) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for _, u := range m.bySession {
		if u.ChannelID == channelID {
			n++
		}
	}
	return n
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

// The manager never hands out *mumble.User. Every record it owns can be mutated
// concurrently by UpdateUser, so a caller holding a pointer would be reading
// through an unsynchronised alias the moment the manager lock is released. All
// reads therefore go through one of the Snapshot* accessors below, which copy
// under the lock (deeply, for the slice fields). Writes go through UpdateUser.

// Snapshot returns a deep copy of a user's record.
func (m *Manager) Snapshot(sessionID uint32) (mumble.User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u := m.bySession[sessionID]
	if u == nil {
		return mumble.User{}, false
	}
	return cloneUser(u), true
}

// SnapshotByName returns a deep copy of the user with the given username.
func (m *Manager) SnapshotByName(name string) (mumble.User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u := m.byName[name]
	if u == nil {
		return mumble.User{}, false
	}
	return cloneUser(u), true
}

// SnapshotByUserID returns a deep copy of the first connected session belonging to
// a registered user ID. Used by the ACL evaluator, which resolves @in/@out/@sub and
// token groups against the user's live channel.
func (m *Manager) SnapshotByUserID(userID uint32) (mumble.User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, u := range m.bySession {
		if u.UserID == userID {
			return cloneUser(u), true
		}
	}
	return mumble.User{}, false
}

// SnapshotAll returns deep copies of every connected user.
func (m *Manager) SnapshotAll() []mumble.User {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]mumble.User, 0, len(m.bySession))
	for _, u := range m.bySession {
		out = append(out, cloneUser(u))
	}
	return out
}

// SnapshotByChannel returns deep copies of the users in the given channel.
func (m *Manager) SnapshotByChannel(channelID uint32) []mumble.User {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []mumble.User
	for _, u := range m.bySession {
		if u.ChannelID == channelID {
			out = append(out, cloneUser(u))
		}
	}
	return out
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
