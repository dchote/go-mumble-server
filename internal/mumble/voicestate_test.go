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

// The regression test for issue #1: murmur mutates the inbound message in place so
// synthesised changes reach the broadcast (Messages.cpp:979-993). A self_deaf=true
// request must gain explicit self_mute presence on us itself, not just on vs.
func TestApplySelfVoiceState_MutatesMessageWithSynthesisedChanges(t *testing.T) {
	vs := mumble.VoiceState{}
	us := &messages.UserState{SelfDeaf: true, SetFields: messages.UserStateSetSelfDeaf}
	applySelfVoiceState(&vs, us)

	if !us.Has(messages.UserStateSetSelfMute) || !us.SelfMute {
		t.Error("self_deaf=true must synthesise an explicit self_mute=true on the message")
	}
}

// Unmuting while deafened must synthesise an explicit self_deaf=false on the message,
// otherwise echo-driven clients (Mumla, Plumble) never learn they were undeafened.
func TestApplySelfVoiceState_UnmuteSynthesisesSelfDeafFalse(t *testing.T) {
	vs := mumble.VoiceState{SelfMute: true, SelfDeaf: true}
	us := &messages.UserState{SetFields: messages.UserStateSetSelfMute} // self_mute=false
	applySelfVoiceState(&vs, us)

	if !us.Has(messages.UserStateSetSelfDeaf) || us.SelfDeaf {
		t.Error("self_mute=false while deafened must synthesise an explicit self_deaf=false")
	}
}

// Undeafening alone must NOT touch self_mute on the message: murmur only synthesises
// self_mute when deafening (true), never when un-deafening, so a client that was both
// muted and deafened stays muted and the broadcast says nothing about mute at all.
func TestApplySelfVoiceState_UndeafenAloneDoesNotTouchSelfMute(t *testing.T) {
	vs := mumble.VoiceState{SelfMute: true, SelfDeaf: true}
	us := &messages.UserState{SetFields: messages.UserStateSetSelfDeaf} // self_deaf=false
	applySelfVoiceState(&vs, us)

	if vs.SelfMute != true {
		t.Error("self_mute must remain true after undeafen-only")
	}
	if us.Has(messages.UserStateSetSelfMute) {
		t.Error("undeafen-only must not add a self_mute presence bit to the message")
	}
}

func TestApplyAdminVoiceState_MutatesMessageWithSynthesisedChanges(t *testing.T) {
	vs := mumble.VoiceState{}
	us := &messages.UserState{Deaf: true, SetFields: messages.UserStateSetDeaf}
	applyAdminVoiceState(&vs, us)

	if !us.Has(messages.UserStateSetMute) || !us.Mute {
		t.Error("deaf=true must synthesise an explicit mute=true on the message")
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

// applyChannelEnterEffects mirrors murmur's userEnterChannel: priority speaker is
// always cleared on a channel move, but only reported as changed if it was actually
// set, matching Server.cpp:2042-2055's `if (p->bPrioritySpeaker) { ...; announce }`.
func TestApplyChannelEnterEffects_ClearsPrioritySpeakerWhenSet(t *testing.T) {
	u := &mumble.User{VoiceState: mumble.VoiceState{PrioritySpeaker: true}}
	priorityChanged, _ := applyChannelEnterEffects(u, true)
	if !priorityChanged || u.PrioritySpeaker {
		t.Error("priority speaker must be cleared and reported as changed")
	}
}

func TestApplyChannelEnterEffects_NoChangeWhenAlreadyClear(t *testing.T) {
	u := &mumble.User{}
	priorityChanged, suppressChanged := applyChannelEnterEffects(u, true)
	if priorityChanged {
		t.Error("priority speaker already false must not be reported as changed")
	}
	if suppressChanged {
		t.Error("suppress must not change when maySpeak already matches !suppress")
	}
}

func TestApplySuppressFromSpeak(t *testing.T) {
	tests := []struct {
		name          string
		startSuppress bool
		maySpeak      bool
		wantSuppress  bool
		wantChanged   bool
	}{
		{"denied speak while unsuppressed flips to suppressed", false, false, true, true},
		{"granted speak while suppressed flips to unsuppressed", true, true, false, true},
		{"already suppressed and denied speak: no change", true, false, true, false},
		{"already unsuppressed and granted speak: no change", false, true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &mumble.User{VoiceState: mumble.VoiceState{Suppress: tt.startSuppress}}
			changed := applySuppressFromSpeak(u, tt.maySpeak)
			if changed != tt.wantChanged || u.Suppress != tt.wantSuppress {
				t.Errorf("changed=%v Suppress=%v, want changed=%v Suppress=%v",
					changed, u.Suppress, tt.wantChanged, tt.wantSuppress)
			}
		})
	}
}

// userToState is a snapshot, not a delta: presence here means "this flag is set", so
// a freshly-connected user with every flag at its zero value must carry no voice
// presence bits at all. Broadcasting all seven regardless of value was the v0.1.4
// regression (every routine update looked like a burst of mute/deaf/suppress/
// priority-speaker/recording changes to clients that log from field presence, e.g.
// the official client's MainWindow::msgUserState).
func TestUserToState_OmitsFalseVoiceFlags(t *testing.T) {
	u := &mumble.User{SessionID: 2, ChannelID: 1}
	state := userToState(*u)

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
		if state.Has(tt.bit) {
			t.Errorf("%s presence bit set on a snapshot with everything false", tt.name)
		}
	}

	// Session and channel_id are always present on a snapshot.
	if !state.Has(messages.UserStateSetSession) || !state.Has(messages.UserStateSetChannelID) {
		t.Error("session/channel_id must always be present on a snapshot")
	}

	// Name and comment stay omit-if-empty to avoid spurious client-side rename and
	// comment-reset events.
	if state.Has(messages.UserStateSetName) || state.Has(messages.UserStateSetComment) {
		t.Error("name/comment should not carry presence bits on routine updates")
	}
}

// True voice flags are emitted, and deaf/mute plus self_deaf/self_mute are mutually
// exclusive on the wire, matching murmur's `if (deaf) ... else if (mute) ...`
// (Messages.cpp:511-524) exactly.
func TestUserToState_EmitsTrueVoiceFlagsWithElseIfExclusivity(t *testing.T) {
	u := &mumble.User{
		SessionID: 2,
		VoiceState: mumble.VoiceState{
			Deaf: true, Mute: true, // deaf implies mute is not separately reported
			SelfDeaf: true, SelfMute: true, // same for self_deaf/self_mute
			Suppress: true, PrioritySpeaker: true, Recording: true,
		},
	}
	state := userToState(*u)

	if !state.Deaf || !state.Has(messages.UserStateSetDeaf) {
		t.Error("deaf must be present and true")
	}
	if state.Mute || state.Has(messages.UserStateSetMute) {
		t.Error("mute must not be separately reported when deaf is true")
	}
	if !state.SelfDeaf || !state.Has(messages.UserStateSetSelfDeaf) {
		t.Error("self_deaf must be present and true")
	}
	if state.SelfMute || state.Has(messages.UserStateSetSelfMute) {
		t.Error("self_mute must not be separately reported when self_deaf is true")
	}
	for _, tt := range []struct {
		name string
		bit  uint32
		val  bool
	}{
		{"suppress", messages.UserStateSetSuppress, state.Suppress},
		{"priority_speaker", messages.UserStateSetPrioritySpeaker, state.PrioritySpeaker},
		{"recording", messages.UserStateSetRecording, state.Recording},
	} {
		if !state.Has(tt.bit) || !tt.val {
			t.Errorf("%s must be present and true", tt.name)
		}
	}
}

// Muted-but-not-deaf and self-muted-but-not-self-deaf take the mute/self_mute branch
// of the else-if.
func TestUserToState_EmitsMuteWithoutDeaf(t *testing.T) {
	u := &mumble.User{
		SessionID:  2,
		VoiceState: mumble.VoiceState{Mute: true, SelfMute: true},
	}
	state := userToState(*u)

	if !state.Mute || !state.Has(messages.UserStateSetMute) {
		t.Error("mute must be present and true")
	}
	if state.Has(messages.UserStateSetDeaf) {
		t.Error("deaf must not be present when false")
	}
	if !state.SelfMute || !state.Has(messages.UserStateSetSelfMute) {
		t.Error("self_mute must be present and true")
	}
	if state.Has(messages.UserStateSetSelfDeaf) {
		t.Error("self_deaf must not be present when false")
	}
}
