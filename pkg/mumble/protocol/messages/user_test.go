package messages

import (
	"bytes"
	"testing"
)

// Mumble.proto is proto2, so an explicitly assigned false must still reach the peer.
// Clients that derive their local mute state from the server echo (Mumla, Plumble)
// rely on this: if a cleared flag is omitted they never learn it was cleared.
func TestUserState_ExplicitFalseSurvivesRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		bit  uint32
		get  func(*UserState) bool
	}{
		{"mute", UserStateSetMute, func(m *UserState) bool { return m.Mute }},
		{"deaf", UserStateSetDeaf, func(m *UserState) bool { return m.Deaf }},
		{"suppress", UserStateSetSuppress, func(m *UserState) bool { return m.Suppress }},
		{"self_mute", UserStateSetSelfMute, func(m *UserState) bool { return m.SelfMute }},
		{"self_deaf", UserStateSetSelfDeaf, func(m *UserState) bool { return m.SelfDeaf }},
		{"priority_speaker", UserStateSetPrioritySpeaker, func(m *UserState) bool { return m.PrioritySpeaker }},
		{"recording", UserStateSetRecording, func(m *UserState) bool { return m.Recording }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Field left at false, but its presence bit is set.
			in := &UserState{Session: 7, SetFields: tt.bit}
			data, err := in.Marshal()
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			var out UserState
			if err := out.Unmarshal(data); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if !out.Has(tt.bit) {
				t.Errorf("presence bit for %s lost; explicit false was not encoded", tt.name)
			}
			if tt.get(&out) {
				t.Errorf("%s = true, want false", tt.name)
			}
		})
	}
}

// allVoiceFieldBits mirrors the seven voice-flag presence bits. It exists only in this
// codec-layer test file (not in the production package) so a policy layer cannot
// accidentally depend on a "set everything" convenience constant again; see the
// delta-vs-snapshot rule in docs/architecture/protocol-encoding.md.
const allVoiceFieldBits = UserStateSetMute | UserStateSetDeaf | UserStateSetSuppress |
	UserStateSetSelfMute | UserStateSetSelfDeaf | UserStateSetPrioritySpeaker |
	UserStateSetRecording

// The whole point of the fix: an unmute broadcast must carry self_mute=false rather
// than omitting the field.
func TestUserState_UnmuteBroadcastCarriesExplicitFalse(t *testing.T) {
	unmuted := &UserState{
		Session:   3,
		SetFields: UserStateSetSession | UserStateSetChannelID | allVoiceFieldBits,
	}
	data, err := unmuted.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// field 9 (self_mute), varint wire type -> tag byte 0x48, value 0x00
	if !bytes.Contains(data, []byte{0x48, 0x00}) {
		t.Errorf("self_mute=false not present on the wire: % x", data)
	}

	var out UserState
	if err := out.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, bit := range []struct {
		name string
		bit  uint32
	}{
		{"mute", UserStateSetMute},
		{"deaf", UserStateSetDeaf},
		{"self_mute", UserStateSetSelfMute},
		{"self_deaf", UserStateSetSelfDeaf},
	} {
		if !out.Has(bit.bit) {
			t.Errorf("%s missing from broadcast", bit.name)
		}
	}
}

// Callers that do not populate SetFields must keep the previous omit-if-empty
// encoding, so presence handling stays opt-in per call site.
func TestUserState_NoPresenceBitsOmitsDefaults(t *testing.T) {
	data, err := (&UserState{Session: 1}).Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Only session (field 1, non-zero). channel_id 0 is omitted without its has-bit.
	want := []byte{0x08, 0x01}
	if !bytes.Equal(data, want) {
		t.Errorf("encoding = % x, want % x", data, want)
	}
}

func TestUserState_OmitsZeroSessionWithoutPresence(t *testing.T) {
	// Client self-mute packets typically omit session. Encoding must not invent session=0.
	data, err := (&UserState{
		SelfMute:  true,
		SetFields: UserStateSetSelfMute,
	}).Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out UserState
	if err := out.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Has(UserStateSetSession) {
		t.Error("session should be absent when unset")
	}
}

func TestUserState_ValuesRoundTrip(t *testing.T) {
	in := &UserState{
		Session:         11,
		Actor:           4,
		Name:            "alice",
		UserID:          2,
		ChannelID:       5,
		SelfMute:        true,
		SelfDeaf:        true,
		PrioritySpeaker: true,
		Comment:         "hi",
		Hash:            "abc",
		SetFields: UserStateSetSession | UserStateSetActor | UserStateSetName |
			UserStateSetUserID | UserStateSetChannelID | allVoiceFieldBits |
			UserStateSetComment | UserStateSetHash,
	}
	data, err := in.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out UserState
	if err := out.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Session != 11 || out.Actor != 4 || out.Name != "alice" || out.UserID != 2 ||
		out.ChannelID != 5 || out.Comment != "hi" || out.Hash != "abc" {
		t.Errorf("scalar round-trip mismatch: got %+v", out)
	}
	if !out.SelfMute || !out.SelfDeaf || !out.PrioritySpeaker {
		t.Errorf("true flags lost: got %+v", out)
	}
	if out.Mute || out.Deaf || out.Suppress || out.Recording {
		t.Errorf("false flags became true: got %+v", out)
	}
}
