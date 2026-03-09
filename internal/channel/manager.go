package channel

import (
	"sort"
	"sync"

	"github.com/dchote/go-mumble-server/internal/database/models"
	"github.com/dchote/go-mumble-server/pkg/mumble"
	"gorm.io/gorm"
)

// Manager maintains the channel tree state.
type Manager struct {
	mu       sync.RWMutex
	db       *gorm.DB
	serverID uint
	tree     map[uint32]*channelNode
}

type channelNode struct {
	*mumble.Channel
	Children []*channelNode
}

// NewManager creates a ChannelManager for a virtual server.
func NewManager(db *gorm.DB, serverID uint) *Manager {
	m := &Manager{
		db:       db,
		serverID: serverID,
		tree:     make(map[uint32]*channelNode),
	}
	m.load()
	return m
}

// Reload re-reads the channel tree from the database. Use when the DB may have
// been modified outside this manager (e.g. by another process or REST when using
// a fallback manager).
func (m *Manager) Reload() {
	m.mu.Lock()
	m.tree = make(map[uint32]*channelNode)
	m.mu.Unlock()
	m.load()
}

func (m *Manager) load() {
	var rows []models.Channel
	if err := m.db.Where("server_id = ?", m.serverID).Order("position, id").Find(&rows).Error; err != nil {
		return
	}
	if len(rows) == 0 {
		root := models.Channel{
			ID:         0,
			ServerID:   m.serverID,
			ParentID:   nil,
			Name:       "Root",
			Position:   0,
			InheritACL: true,
		}
		if err := m.db.Select("ID", "ServerID", "ParentID", "Name", "Position", "InheritACL").Create(&root).Error; err == nil {
			rows = append(rows, root)
		}
	} else {
		rows = m.fixRootID(rows)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range rows {
		nc := &channelNode{
			Channel: &mumble.Channel{
				ID:          uint32(c.ID),
				ParentID:    ptrToUint32(c.ParentID),
				Name:        c.Name,
				Description: c.Description,
				Position:    c.Position,
				MaxUsers:    c.MaxUsers,
				IsTemporary: c.IsTemporary,
				Links:       c.Links,
				InheritACL:  c.InheritACL,
			},
			Children: nil,
		}
		m.tree[uint32(c.ID)] = nc
	}
	for _, nc := range m.tree {
		if nc.ID == 0 {
			continue // root has no parent
		}
		if p := m.tree[nc.ParentID]; p != nil {
			p.Children = append(p.Children, nc)
		}
	}
	for _, nc := range m.tree {
		sort.Slice(nc.Children, func(i, j int) bool {
			return nc.Children[i].Position < nc.Children[j].Position
		})
	}
}

// fixRootID migrates the root channel to ID 0 if a pre-existing database has
// it at a different ID (caused by GORM auto-increment before the Select fix).
func (m *Manager) fixRootID(rows []models.Channel) []models.Channel {
	var rootIdx int = -1
	hasZero := false
	for i, c := range rows {
		if c.ParentID == nil {
			rootIdx = i
		}
		if c.ID == 0 {
			hasZero = true
		}
	}
	if rootIdx == -1 || rows[rootIdx].ID == 0 {
		return rows // root already at 0 or no root found
	}
	if hasZero {
		return rows // ID 0 already taken by another row; don't migrate
	}

	oldID := rows[rootIdx].ID

	// Use raw SQL: SQLite doesn't allow PK updates through GORM easily.
	tx := m.db.Begin()
	// 1. Update children pointing to old root
	tx.Exec("UPDATE channels SET parent_id = 0 WHERE server_id = ? AND parent_id = ?", m.serverID, oldID)
	// 2. Update ACLs and groups referencing old root
	tx.Exec("UPDATE channel_acls SET channel_id = 0 WHERE server_id = ? AND channel_id = ?", m.serverID, oldID)
	tx.Exec("UPDATE channel_groups SET channel_id = 0 WHERE server_id = ? AND channel_id = ?", m.serverID, oldID)
	// 3. Change root row ID: insert new, delete old
	tx.Exec("INSERT INTO channels (id, server_id, parent_id, name, description, position, max_users, is_temporary, inherit_acl, links, created_at, updated_at) SELECT 0, server_id, parent_id, name, description, position, max_users, is_temporary, inherit_acl, links, created_at, updated_at FROM channels WHERE id = ? AND server_id = ?", oldID, m.serverID)
	tx.Exec("DELETE FROM channels WHERE id = ? AND server_id = ?", oldID, m.serverID)
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return rows
	}

	// Reload from DB after migration
	var fixed []models.Channel
	if err := m.db.Where("server_id = ?", m.serverID).Order("position, id").Find(&fixed).Error; err != nil {
		return rows
	}
	return fixed
}

func ptrToUint32(p *uint) uint32 {
	if p == nil {
		return 0
	}
	return uint32(*p)
}

// GetChannel returns a channel by ID.
func (m *Manager) GetChannel(id uint32) (*mumble.Channel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.tree[id]
	if !ok {
		return nil, false
	}
	return n.Channel, true
}

// GetTree returns the full channel tree as a flat list (BFS order).
func (m *Manager) GetTree() []*mumble.Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*mumble.Channel
	var walk func(n *channelNode)
	walk = func(n *channelNode) {
		if n == nil {
			return
		}
		out = append(out, n.Channel)
		for _, c := range n.Children {
			walk(c)
		}
	}
	root := m.tree[0]
	if root == nil {
		for _, n := range m.tree {
			if n.ParentID == 0 {
				root = n
				break
			}
		}
	}
	walk(root)
	return out
}

// RootID returns the root channel ID (0 per Mumble spec).
func (m *Manager) RootID() uint32 {
	return 0
}

// ServerID returns the virtual server ID.
func (m *Manager) ServerID() uint {
	return m.serverID
}

// AncestorChain returns channel IDs from the given channel up to root (inclusive).
// First element is the channel, last is root.
func (m *Manager) AncestorChain(channelID uint32) []uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var chain []uint32
	cid := channelID
	for {
		n, ok := m.tree[cid]
		if !ok {
			break
		}
		chain = append(chain, cid)
		if n.ParentID == 0 && cid != 0 {
			break
		}
		if n.ParentID == cid {
			break
		}
		cid = n.ParentID
		if cid == 0 {
			break
		}
	}
	return chain
}

// GetChannelWithMeta returns the channel and its InheritACL. ok is false if not found.
func (m *Manager) GetChannelWithMeta(channelID uint32) (ch *mumble.Channel, inheritACL bool, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.tree[channelID]
	if !ok {
		return nil, false, false
	}
	return n.Channel, n.InheritACL, true
}

// Create creates a new channel. Returns the channel or nil on error.
func (m *Manager) Create(parentID uint32, name string, description string, position int32, temporary bool, maxUsers uint32) *mumble.Channel {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tree[parentID]; !ok {
		return nil
	}
	if name == "" {
		return nil
	}
	row := models.Channel{
		ServerID:    m.serverID,
		ParentID:    ptrUint(parentID),
		Name:        name,
		Description: description,
		Position:    position,
		MaxUsers:    maxUsers,
		IsTemporary: temporary,
	}
	if err := m.db.Create(&row).Error; err != nil {
		return nil
	}
	ch := &mumble.Channel{
		ID:          uint32(row.ID),
		ParentID:    parentID,
		Name:        row.Name,
		Description: row.Description,
		Position:    row.Position,
		MaxUsers:    row.MaxUsers,
		IsTemporary: row.IsTemporary,
		Links:       nil,
		InheritACL:  row.InheritACL,
	}
	m.tree[ch.ID] = &channelNode{Channel: ch, Children: nil}
	if p := m.tree[parentID]; p != nil {
		p.Children = append(p.Children, m.tree[ch.ID])
		sort.Slice(p.Children, func(i, j int) bool {
			return p.Children[i].Position < p.Children[j].Position
		})
	}
	return ch
}

func ptrUint(v uint32) *uint { u := uint(v); return &u }

// UpdateOpts holds optional channel updates (nil = don't change).
type UpdateOpts struct {
	Name        *string
	Description *string
	Position    *int32
	MaxUsers    *uint32
	Temporary   *bool
	Links       []uint32 // if non-nil, replace
	LinksAdd    []uint32
	LinksRemove []uint32
}

// Update updates channel metadata. Returns false if channel not found or update failed.
func (m *Manager) Update(id uint32, opts UpdateOpts) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.tree[id]
	if !ok {
		return false
	}
	updates := map[string]interface{}{}
	if opts.Name != nil {
		n.Name = *opts.Name
		updates["name"] = *opts.Name
	}
	if opts.Description != nil {
		n.Description = *opts.Description
		updates["description"] = *opts.Description
	}
	if opts.Position != nil {
		n.Position = *opts.Position
		updates["position"] = *opts.Position
	}
	if opts.MaxUsers != nil {
		n.MaxUsers = *opts.MaxUsers
		updates["max_users"] = *opts.MaxUsers
	}
	if opts.Temporary != nil {
		n.IsTemporary = *opts.Temporary
		updates["is_temporary"] = *opts.Temporary
	}
	if opts.Links != nil {
		n.Links = opts.Links
		updates["links"] = models.Uint32Slice(opts.Links)
	} else if len(opts.LinksAdd) > 0 || len(opts.LinksRemove) > 0 {
		links := make(map[uint32]bool)
		for _, id := range n.Links {
			links[id] = true
		}
		for _, id := range opts.LinksRemove {
			delete(links, id)
		}
		for _, id := range opts.LinksAdd {
			links[id] = true
		}
		n.Links = nil
		for id := range links {
			n.Links = append(n.Links, id)
		}
		updates["links"] = models.Uint32Slice(n.Links)
	}
	if len(updates) > 0 {
		if err := m.db.Model(&models.Channel{}).Where("id = ? AND server_id = ?", id, m.serverID).Updates(updates).Error; err != nil {
			return false
		}
	}
	return true
}

// Remove removes a channel. Returns false if channel not found, is root, or has children.
func (m *Manager) Remove(id uint32) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id == 0 {
		return false
	}
	n, ok := m.tree[id]
	if !ok {
		return false
	}
	if len(n.Children) > 0 {
		return false
	}
	if err := m.db.Where("id = ? AND server_id = ?", id, m.serverID).Delete(&models.Channel{}).Error; err != nil {
		return false
	}
	delete(m.tree, id)
	if p := m.tree[n.ParentID]; p != nil {
		for i, c := range p.Children {
			if c.ID == id {
				p.Children = append(p.Children[:i], p.Children[i+1:]...)
				break
			}
		}
	}
	return true
}

// LinkedChannelIDs returns the channel ID and its linked channel IDs.
func (m *Manager) LinkedChannelIDs(channelID uint32) []uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.tree[channelID]
	if !ok {
		return nil
	}
	out := make([]uint32, 0, 1+len(n.Links))
	out = append(out, channelID)
	for _, lid := range n.Links {
		if m.tree[lid] != nil {
			out = append(out, lid)
		}
	}
	return out
}

// SubtreeIDs returns all channel IDs in the subtree rooted at id (including id).
func (m *Manager) SubtreeIDs(id uint32) []uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []uint32
	var walk func(uint32)
	walk = func(cid uint32) {
		out = append(out, cid)
		if n := m.tree[cid]; n != nil {
			for _, ch := range n.Children {
				walk(ch.ID)
			}
		}
	}
	walk(id)
	return out
}

// CleanEmptyTempChannels removes temporary channels that have no users and no children.
// hasUsers returns true if the channel has any users.
func (m *Manager) CleanEmptyTempChannels(hasUsers func(channelID uint32) bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	removed := true
	for removed {
		removed = false
		for id, n := range m.tree {
			if id == 0 || !n.IsTemporary || len(n.Children) > 0 {
				continue
			}
			if hasUsers(id) {
				continue
			}
			parentID := n.ParentID
			if err := m.db.Where("id = ? AND server_id = ?", id, m.serverID).Delete(&models.Channel{}).Error; err != nil {
				continue
			}
			delete(m.tree, id)
			if p := m.tree[parentID]; p != nil {
				for i, ch := range p.Children {
					if ch.ID == id {
						p.Children = append(p.Children[:i], p.Children[i+1:]...)
						break
					}
				}
			}
			removed = true
			break
		}
	}
}

// NextID returns the next unused channel ID for temporary channels (in-memory only).
func (m *Manager) NextID() uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	max := uint32(0)
	for id := range m.tree {
		if id > max {
			max = id
		}
	}
	return max + 1
}
