package user

import (
	"bytes"
	"testing"

	"github.com/dchote/go-mumble-server/pkg/mumble"
)

func TestSnapshotDeepCopiesSlices(t *testing.T) {
	m := NewManager(nil, 10)
	u := &mumble.User{
		Name:          "alice",
		Texture:       []byte{1, 2, 3},
		PluginContext: []byte{4, 5},
		AccessTokens:  []string{"tok"},
		VoiceState:    mumble.VoiceState{SelfMute: true},
	}
	if !m.Add(u) {
		t.Fatal("Add failed")
	}

	snap, ok := m.Snapshot(u.SessionID)
	if !ok {
		t.Fatal("Snapshot failed")
	}
	snap.Texture[0] = 9
	snap.AccessTokens[0] = "mutated"
	snap.SelfMute = false

	live, _ := m.Snapshot(u.SessionID)
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

func TestSpeakGateFor(t *testing.T) {
	m := NewManager(nil, 10)
	u := &mumble.User{
		Name:       "bob",
		UserID:     7,
		ChannelID:  3,
		VoiceState: mumble.VoiceState{Suppress: true, SelfMute: true},
	}
	if !m.Add(u) {
		t.Fatal("Add failed")
	}
	g, ok := m.SpeakGateFor(u.SessionID)
	if !ok {
		t.Fatal("SpeakGateFor failed")
	}
	if g.UserID != 7 || g.ChannelID != 3 || !g.Suppress || !g.SelfMute {
		t.Errorf("SpeakGate = %+v", g)
	}
}
