package user

import (
	"bytes"
	"testing"

	"github.com/dchote/go-mumble-server/pkg/mumble"
)

func TestSnapshotDeepCopiesSlices(t *testing.T) {
	m := NewManager(nil, 10)
	stored, ok := m.Add(mumble.User{
		Name:          "alice",
		Texture:       []byte{1, 2, 3},
		PluginContext: []byte{4, 5},
		AccessTokens:  []string{"tok"},
		VoiceState:    mumble.VoiceState{SelfMute: true},
	})
	if !ok {
		t.Fatal("Add failed")
	}

	snap, ok := m.Snapshot(stored.SessionID)
	if !ok {
		t.Fatal("Snapshot failed")
	}
	snap.Texture[0] = 9
	snap.AccessTokens[0] = "mutated"
	snap.SelfMute = false

	live, _ := m.Snapshot(stored.SessionID)
	if live.Texture[0] != 1 {
		t.Error("Snapshot must deep-copy Texture")
	}
	if live.AccessTokens[0] != "tok" {
		t.Error("Snapshot must deep-copy AccessTokens")
	}
	if !live.SelfMute {
		t.Error("mutating Snapshot.VoiceState must not affect the live user")
	}
	if !bytes.Equal(live.PluginContext, []byte{4, 5}) {
		t.Error("Snapshot must deep-copy PluginContext")
	}
}

// Add must not retain the caller's slices either: the caller still owns the value
// it passed in, and mutating it afterwards must not reach the stored record.
func TestAddDeepCopiesCallerSlices(t *testing.T) {
	m := NewManager(nil, 10)
	texture := []byte{1, 2, 3}
	stored, ok := m.Add(mumble.User{Name: "carol", Texture: texture})
	if !ok {
		t.Fatal("Add failed")
	}
	texture[0] = 9
	stored.Texture[1] = 9

	live, _ := m.Snapshot(stored.SessionID)
	if !bytes.Equal(live.Texture, []byte{1, 2, 3}) {
		t.Errorf("stored texture = %v, want [1 2 3]", live.Texture)
	}
}

func TestSnapshotAllAndByChannel(t *testing.T) {
	m := NewManager(nil, 10)
	a, _ := m.Add(mumble.User{Name: "a", ChannelID: 1})
	b, _ := m.Add(mumble.User{Name: "b", ChannelID: 2})

	if got := len(m.SnapshotAll()); got != 2 {
		t.Errorf("SnapshotAll len = %d, want 2", got)
	}
	inChan2 := m.SnapshotByChannel(2)
	if len(inChan2) != 1 || inChan2[0].SessionID != b.SessionID {
		t.Errorf("SnapshotByChannel(2) = %+v, want just %d", inChan2, b.SessionID)
	}
	if got := m.CountInChannel(1); got != 1 {
		t.Errorf("CountInChannel(1) = %d, want 1", got)
	}
	if !m.Exists(a.SessionID) {
		t.Error("Exists must find an added session")
	}
	if _, ok := m.SnapshotByName("a"); !ok {
		t.Error("SnapshotByName must find an added user")
	}

	// Snapshots are copies: mutating one must not touch the manager's record.
	all := m.SnapshotAll()
	all[0].Name = "mutated"
	if _, ok := m.SnapshotByName("mutated"); ok {
		t.Error("mutating a snapshot leaked into the manager")
	}
}

func TestSnapshotByUserID(t *testing.T) {
	m := NewManager(nil, 10)
	stored, _ := m.Add(mumble.User{Name: "reg", UserID: 42, ChannelID: 5})

	got, ok := m.SnapshotByUserID(42)
	if !ok || got.SessionID != stored.SessionID || got.ChannelID != 5 {
		t.Errorf("SnapshotByUserID(42) = %+v, %v", got, ok)
	}
	if _, ok := m.SnapshotByUserID(43); ok {
		t.Error("SnapshotByUserID must not match an unknown user ID")
	}
}

func TestSpeakGateFor(t *testing.T) {
	m := NewManager(nil, 10)
	stored, ok := m.Add(mumble.User{
		Name:       "bob",
		UserID:     7,
		ChannelID:  3,
		VoiceState: mumble.VoiceState{Suppress: true, SelfMute: true},
	})
	if !ok {
		t.Fatal("Add failed")
	}
	g, ok := m.SpeakGateFor(stored.SessionID)
	if !ok {
		t.Fatal("SpeakGateFor failed")
	}
	if g.UserID != 7 || g.ChannelID != 3 || !g.Suppress || !g.SelfMute {
		t.Errorf("SpeakGate = %+v", g)
	}
}

func TestRemoveReturnsCopy(t *testing.T) {
	m := NewManager(nil, 10)
	stored, _ := m.Add(mumble.User{Name: "dave", ChannelID: 4})

	removed, ok := m.Remove(stored.SessionID)
	if !ok || removed.Name != "dave" || removed.ChannelID != 4 {
		t.Errorf("Remove = %+v, %v", removed, ok)
	}
	if m.Exists(stored.SessionID) {
		t.Error("session still present after Remove")
	}
	if _, ok := m.Remove(stored.SessionID); ok {
		t.Error("second Remove must report not found")
	}
}
