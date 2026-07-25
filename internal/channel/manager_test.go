package channel

import (
	"path/filepath"
	"testing"

	"github.com/dchote/go-mumble-server/internal/database"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "channels.sqlite"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

// The protocol pins root at channel 0, and RootID() hard-codes it. A root created
// anywhere else is invisible to every root-scoped lookup — most damagingly the ACL
// checks for ban, kick and register, which then silently fall back to defaults.
func TestNewManager_CreatesRootAtChannelZero(t *testing.T) {
	m := NewManager(newTestDB(t), 1)

	root, ok := m.GetChannel(m.RootID())
	if !ok {
		t.Fatalf("no channel at RootID %d; tree = %+v", m.RootID(), m.GetTree())
	}
	if root.Name != "Root" {
		t.Errorf("root name = %q, want %q", root.Name, "Root")
	}
	if len(m.GetTree()) != 1 {
		t.Errorf("fresh server has %d channels, want only root", len(m.GetTree()))
	}
}

// A root that a previous version left at a non-zero ID is migrated on load, taking
// its children and ACLs with it.
func TestNewManager_MigratesLegacyRootToChannelZero(t *testing.T) {
	db := newTestDB(t)
	if err := db.Exec(
		"INSERT INTO channels (id, server_id, parent_id, name, position, inherit_acl) VALUES (7, 1, NULL, 'Root', 0, 1)",
	).Error; err != nil {
		t.Fatalf("seed legacy root: %v", err)
	}
	if err := db.Exec(
		"INSERT INTO channels (id, server_id, parent_id, name, position, inherit_acl) VALUES (8, 1, 7, 'Lobby', 0, 1)",
	).Error; err != nil {
		t.Fatalf("seed child: %v", err)
	}

	m := NewManager(db, 1)

	if _, ok := m.GetChannel(0); !ok {
		t.Fatalf("legacy root was not migrated to 0; tree = %+v", m.GetTree())
	}
	if _, ok := m.GetChannel(7); ok {
		t.Error("legacy root ID is still present after migration")
	}
	child, ok := m.GetChannel(8)
	if !ok {
		t.Fatal("child channel lost during migration")
	}
	if child.ParentID != 0 {
		t.Errorf("child parent = %d, want the migrated root 0", child.ParentID)
	}
}

// Clients build their channel tree in the order the server announces it. Mumla's
// protocol library resolves a ChannelState's parent immediately and throws on a
// parent it has not seen (humla ModelHandler.messageChannelState), which drops the
// connection, so a child must never be announced before its parent.
func TestGetTree_AnnouncesParentsBeforeChildren(t *testing.T) {
	m := NewManager(newTestDB(t), 1)
	lobby := m.Create(m.RootID(), "lobby", "", 1, false, 0)
	games := m.Create(m.RootID(), "games", "", 0, false, 0)
	if lobby == nil || games == nil {
		t.Fatal("create top-level channels")
	}
	nested := m.Create(lobby.ID, "nested", "", 0, false, 0)
	deeper := m.Create(games.ID, "deeper", "", 0, false, 0)
	if nested == nil || deeper == nil {
		t.Fatal("create nested channels")
	}

	tree := m.GetTree()
	if len(tree) != 5 {
		t.Fatalf("tree has %d channels, want 5", len(tree))
	}
	if tree[0].ID != m.RootID() {
		t.Errorf("first announced channel = %d, want root %d", tree[0].ID, m.RootID())
	}
	seen := map[uint32]bool{}
	for _, ch := range tree {
		if ch.ID != m.RootID() && !seen[ch.ParentID] {
			t.Errorf("channel %q (%d) announced before its parent %d", ch.Name, ch.ID, ch.ParentID)
		}
		seen[ch.ID] = true
	}
}

// Update rewrites the stored channels in place, so accessors must hand out copies
// rather than the pointers the tree holds.
func TestGetChannel_ReturnsACopy(t *testing.T) {
	m := NewManager(newTestDB(t), 1)
	lobby := m.Create(m.RootID(), "lobby", "", 0, false, 0)
	if lobby == nil {
		t.Fatal("create lobby")
	}
	if !m.Update(lobby.ID, UpdateOpts{Links: []uint32{m.RootID()}}) {
		t.Fatal("link lobby to root")
	}

	got, ok := m.GetChannel(lobby.ID)
	if !ok {
		t.Fatal("lobby disappeared")
	}
	got.Name = "renamed by the caller"
	got.Links[0] = 999

	again, _ := m.GetChannel(lobby.ID)
	if again.Name != "lobby" {
		t.Errorf("stored name = %q, want %q: the caller mutated manager state", again.Name, "lobby")
	}
	if len(again.Links) != 1 || again.Links[0] != m.RootID() {
		t.Errorf("stored links = %v, want [%d]: the links slice is shared", again.Links, m.RootID())
	}
}

// ACL inheritance walks this chain, so it has to reach root. Every direct child of
// root reports ParentID 0 — the same value root itself reports — which makes the
// terminating condition easy to get wrong and silently drops root's ACLs.
func TestAncestorChain_ReachesRoot(t *testing.T) {
	m := NewManager(newTestDB(t), 1)
	lobby := m.Create(m.RootID(), "lobby", "", 0, false, 0)
	if lobby == nil {
		t.Fatal("create lobby")
	}
	nested := m.Create(lobby.ID, "nested", "", 0, false, 0)
	if nested == nil {
		t.Fatal("create nested")
	}

	got := m.AncestorChain(nested.ID)
	want := []uint32{nested.ID, lobby.ID, m.RootID()}
	if len(got) != len(want) {
		t.Fatalf("chain = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("chain = %v, want %v (target first, root last)", got, want)
		}
	}

	if chain := m.AncestorChain(m.RootID()); len(chain) != 1 || chain[0] != m.RootID() {
		t.Errorf("chain from root = %v, want just the root", chain)
	}
	if chain := m.AncestorChain(999); len(chain) != 0 {
		t.Errorf("chain for unknown channel = %v, want empty", chain)
	}
}
