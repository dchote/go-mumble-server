package ban

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dchote/go-mumble-server/internal/database/models"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol/messages"
	"gorm.io/gorm"
)

// Manager maintains the server ban list.
type Manager struct {
	mu       sync.RWMutex
	db       *gorm.DB
	serverID uint
	bans     []banEntry
}

type banEntry struct {
	addr     []byte
	mask     uint32
	hash     string
	name     string
	reason   string
	start    time.Time
	duration uint32
	active   bool
}

// NewManager creates a BanManager.
func NewManager(db *gorm.DB, serverID uint) *Manager {
	m := &Manager{db: db, serverID: serverID}
	m.load()
	return m
}

func (m *Manager) load() {
	var rows []models.Ban
	if err := m.db.Where("server_id = ?", m.serverID).Find(&rows).Error; err != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bans = make([]banEntry, 0, len(rows))
	for _, b := range rows {
		m.bans = append(m.bans, banEntry{
			addr:     b.Address,
			mask:     b.Mask,
			hash:     strings.ToLower(b.Hash),
			name:     b.Name,
			reason:   b.Reason,
			start:    b.Start,
			duration: b.Duration,
			active:   true,
		})
	}
}

// IsBanned returns true if the address or certificate hash is banned.
func (m *Manager) IsBanned(addr net.Addr, certHash string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hash := strings.ToLower(certHash)
	for _, b := range m.bans {
		if !b.active {
			continue
		}
		if b.hash != "" && hash != "" && b.hash == hash {
			return true
		}
		if len(b.addr) > 0 {
			ipStr := addr.String()
			if host, _, err := net.SplitHostPort(ipStr); err == nil {
				ipStr = host
			}
			ip := net.ParseIP(ipStr)
			if ip != nil && matchAddr(ip, b.addr, b.mask) {
				return true
			}
		}
	}
	return false
}

// List returns all active bans as BanEntry messages.
func (m *Manager) List() []messages.BanEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []messages.BanEntry
	for _, b := range m.bans {
		if !b.active {
			continue
		}
		be := messages.BanEntry{
			Address:  b.addr,
			Mask:     b.mask,
			Name:     b.name,
			Hash:     b.hash,
			Reason:   b.reason,
			Start:    b.start.Format(time.RFC3339),
			Duration: b.duration,
		}
		out = append(out, be)
	}
	return out
}

// Replace replaces the ban list with the given entries. Caller must have Ban permission.
func (m *Manager) Replace(bans []messages.BanEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.db.Where("server_id = ?", m.serverID).Delete(&models.Ban{}).Error; err != nil {
		return err
	}
	for _, be := range bans {
		var start time.Time
		if be.Start != "" {
			start, _ = time.Parse(time.RFC3339, be.Start)
		} else {
			start = time.Now()
		}
		row := models.Ban{
			ServerID: m.serverID,
			Address:  be.Address,
			Mask:     be.Mask,
			Name:     be.Name,
			Hash:     strings.ToLower(be.Hash),
			Reason:   be.Reason,
			Start:    start,
			Duration: be.Duration,
		}
		if err := m.db.Create(&row).Error; err != nil {
			return err
		}
	}
	m.load()
	return nil
}

func matchAddr(ip net.IP, addr []byte, mask uint32) bool {
	ip4 := ip.To4()
	if ip4 != nil && len(addr) >= 4 {
		for i := 0; i < 4 && mask > 0; i++ {
			m := uint32(0xff)
			if mask < 8 {
				m = (1 << mask) - 1
			}
			if (addr[i] & byte(m)) != (ip4[i] & byte(m)) {
				return false
			}
			if mask >= 8 {
				mask -= 8
			} else {
				mask = 0
			}
		}
		return true
	}
	return false
}
