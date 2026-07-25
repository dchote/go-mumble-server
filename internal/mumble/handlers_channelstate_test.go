package mumble

import (
	"bytes"
	"testing"

	"github.com/dchote/go-mumble-server/pkg/mumble"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol/wire"
)

// Root is the one channel with no parent, and the proto2 parent field must be
// absent rather than 0 — 0 is root's own ID. Mumla's protocol library looks the
// parent up the moment a ChannelState carries one and dereferences the result
// without a null check (humla ModelHandler.messageChannelState), so a root that
// claims a parent kills the client mid-sync, which is what issue #1's follow-up
// reported as "Mumla abruptly disconnects".
func TestChannelToState_RootOmitsParent(t *testing.T) {
	state := channelToState(&mumble.Channel{ID: 0, Name: "Root"})

	if state.HasParent {
		t.Errorf("root announced parent %d; the field must be absent for root", state.Parent)
	}
	if state.ChannelID != 0 {
		t.Errorf("root channel ID = %d, want 0", state.ChannelID)
	}

	payload, err := state.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if fieldPresentOnWire(t, payload, 2) {
		t.Error("marshaled root ChannelState still carries field 2 (parent); Mumla would NPE")
	}
}

func TestChannelToState_ChildCarriesItsParent(t *testing.T) {
	state := channelToState(&mumble.Channel{ID: 4, ParentID: 0, Name: "lobby"})

	if !state.HasParent {
		t.Fatal("child omitted its parent, so clients cannot place it in the tree")
	}
	if state.Parent != 0 {
		t.Errorf("parent = %d, want root 0", state.Parent)
	}

	payload, err := state.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !fieldPresentOnWire(t, payload, 2) {
		t.Error("marshaled child ChannelState omitted field 2 (parent)")
	}
}

// fieldPresentOnWire reports whether a protobuf field number appears in payload.
func fieldPresentOnWire(t *testing.T, payload []byte, fieldNum int) bool {
	t.Helper()
	b := payload
	for len(b) > 0 {
		fn, wt, n, err := wire.ReadTag(b)
		if err != nil {
			t.Fatalf("ReadTag: %v (payload=%x)", err, payload)
		}
		b = b[n:]
		if fn == fieldNum {
			return true
		}
		skip, err := wire.SkipField(b, wt)
		if err != nil {
			t.Fatalf("SkipField: %v", err)
		}
		b = b[skip:]
	}
	// Silence unused import if bytes is only needed for clarity elsewhere.
	_ = bytes.Equal
	return false
}
