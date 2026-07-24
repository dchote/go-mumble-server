package mumble

import (
	"testing"

	"github.com/dchote/go-mumble-server/pkg/mumble"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol/messages"
)

// Expectations are taken from murmur's Server::msgUserState
// (research/mumble/src/murmur/Messages.cpp). The invariant is that a deafened user is
// always also muted; un-muting clears deafen, but un-deafening leaves mute alone.
func TestApplySelfVoiceState(t *testing.T) {
	const (
		muteBit = messages.UserStateSetSelfMute
		deafBit = messages.UserStateSetSelfDeaf
	)
	tests := []struct {
		name                 string
		startMute, startDeaf bool
		req                  *messages.UserState
		wantMute, wantDeaf   bool
	}{
		{
			name: "self_deaf true implies self_mute",
			req:  &messages.UserState{SelfDeaf: true, SetFields: deafBit},
			// murmur synthesises self_mute=true, which then applies to server state
			wantMute: true, wantDeaf: true,
		},
		{
			name:      "self_deaf false alone leaves self_mute set",
			startMute: true, startDeaf: true,
			req:      &messages.UserState{SetFields: deafBit},
			wantMute: true, wantDeaf: false,
		},
		{
			name:      "self_mute false clears self_deaf",
			startMute: true, startDeaf: true,
			req:      &messages.UserState{SetFields: muteBit},
			wantMute: false, wantDeaf: false,
		},
		{
			name:     "self_mute true alone",
			req:      &messages.UserState{SelfMute: true, SetFields: muteBit},
			wantMute: true, wantDeaf: false,
		},
		{
			name:      "both false is a full unmute",
			startMute: true, startDeaf: true,
			req:      &messages.UserState{SetFields: muteBit | deafBit},
			wantMute: false, wantDeaf: false,
		},
		{
			name:      "mute only, from deafened",
			startMute: true, startDeaf: true,
			req:      &messages.UserState{SelfMute: true, SetFields: muteBit | deafBit},
			wantMute: true, wantDeaf: false,
		},
		{
			name:     "both true is a deafen",
			req:      &messages.UserState{SelfMute: true, SelfDeaf: true, SetFields: muteBit | deafBit},
			wantMute: true, wantDeaf: true,
		},
		{
			name:     "contradictory request resolves in favour of deafen",
			req:      &messages.UserState{SelfMute: false, SelfDeaf: true, SetFields: muteBit | deafBit},
			wantMute: true, wantDeaf: true,
		},
		{
			name:      "absent fields change nothing",
			startMute: true, startDeaf: true,
			req:      &messages.UserState{},
			wantMute: true, wantDeaf: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vs := mumble.VoiceState{SelfMute: tt.startMute, SelfDeaf: tt.startDeaf}
			applySelfVoiceState(&vs, tt.req)
			if vs.SelfMute != tt.wantMute || vs.SelfDeaf != tt.wantDeaf {
				t.Errorf("SelfMute=%v SelfDeaf=%v, want SelfMute=%v SelfDeaf=%v",
					vs.SelfMute, vs.SelfDeaf, tt.wantMute, tt.wantDeaf)
			}
			if vs.SelfDeaf && !vs.SelfMute {
				t.Error("invariant violated: deafened but not muted")
			}
		})
	}
}

func TestApplyAdminVoiceState(t *testing.T) {
	const (
		muteBit = messages.UserStateSetMute
		deafBit = messages.UserStateSetDeaf
	)
	tests := []struct {
		name                 string
		startMute, startDeaf bool
		req                  *messages.UserState
		wantMute, wantDeaf   bool
	}{
		{
			name:     "deaf true implies mute",
			req:      &messages.UserState{Deaf: true, SetFields: deafBit},
			wantMute: true, wantDeaf: true,
		},
		{
			name:      "mute false clears deaf",
			startMute: true, startDeaf: true,
			req:      &messages.UserState{SetFields: muteBit},
			wantMute: false, wantDeaf: false,
		},
		{
			name:      "deaf false alone leaves mute set",
			startMute: true, startDeaf: true,
			req:      &messages.UserState{SetFields: deafBit},
			wantMute: true, wantDeaf: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vs := mumble.VoiceState{Mute: tt.startMute, Deaf: tt.startDeaf}
			applyAdminVoiceState(&vs, tt.req)
			if vs.Mute != tt.wantMute || vs.Deaf != tt.wantDeaf {
				t.Errorf("Mute=%v Deaf=%v, want Mute=%v Deaf=%v",
					vs.Mute, vs.Deaf, tt.wantMute, tt.wantDeaf)
			}
			if vs.Deaf && !vs.Mute {
				t.Error("invariant violated: deafened but not muted")
			}
		})
	}
}

func TestApplyAdminVoiceState_SuppressAndPrioritySpeaker(t *testing.T) {
	vs := mumble.VoiceState{Suppress: true}
	applyAdminVoiceState(&vs, &messages.UserState{
		PrioritySpeaker: true,
		SetFields:       messages.UserStateSetSuppress | messages.UserStateSetPrioritySpeaker,
	})
	if vs.Suppress {
		t.Error("Suppress should have been cleared")
	}
	if !vs.PrioritySpeaker {
		t.Error("PrioritySpeaker should have been set")
	}
}

// A self-muted user's audio must not be relayed, matching murmur's voice gate.
func TestUserToState_BroadcastsClearedFlagsExplicitly(t *testing.T) {
	u := &mumble.User{SessionID: 2, ChannelID: 1}
	state := userToState(u)

	for _, tt := range []struct {
		name string
		bit  uint32
	}{
		{"mute", messages.UserStateSetMute},
		{"deaf", messages.UserStateSetDeaf},
		{"suppress", messages.UserStateSetSuppress},
		{"self_mute", messages.UserStateSetSelfMute},
		{"self_deaf", messages.UserStateSetSelfDeaf},
		{"priority_speaker", messages.UserStateSetPrioritySpeaker},
		{"recording", messages.UserStateSetRecording},
	} {
		if !state.Has(tt.bit) {
			t.Errorf("%s presence bit not set on broadcast state", tt.name)
		}
	}

	// Name and comment stay omit-if-empty to avoid spurious client-side rename and
	// comment-reset events.
	if state.Has(messages.UserStateSetName) || state.Has(messages.UserStateSetComment) {
		t.Error("name/comment should not carry presence bits on routine updates")
	}
}
