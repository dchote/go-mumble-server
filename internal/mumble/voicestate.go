package mumble

import (
	"github.com/dchote/go-mumble-server/pkg/mumble"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol/messages"
)

// The mute and deafen flags are not independent. Mumble's invariant is that a
// deafened user is always also muted, so a UserState that changes one may imply a
// change to the other. Both helpers below reproduce the evaluation order used by
// murmur's Server::msgUserState (src/murmur/Messages.cpp), which is load-bearing:
// deafen is handled first and may synthesise a mute request that the following step
// then applies, and that is what makes deafening imply muting.
//
// Note the asymmetry. Un-muting clears deafen, but un-deafening deliberately leaves
// mute alone: clients decide for themselves whether undeafening should also unmute
// (the official client has an "unmute on undeaf" setting) and send both fields
// explicitly when it should.

// applySelfVoiceState applies a client's own mute/deafen request to vs.
func applySelfVoiceState(vs *mumble.VoiceState, us *messages.UserState) {
	selfMute, selfMuteSet := us.SelfMute, us.Has(messages.UserStateSetSelfMute)

	if us.Has(messages.UserStateSetSelfDeaf) {
		vs.SelfDeaf = us.SelfDeaf
		if vs.SelfDeaf {
			selfMute, selfMuteSet = true, true
		}
	}
	if selfMuteSet {
		vs.SelfMute = selfMute
		if !vs.SelfMute {
			vs.SelfDeaf = false
		}
	}

	if us.Has(messages.UserStateSetRecording) {
		vs.Recording = us.Recording
	}
}

// applyAdminVoiceState applies administratively imposed voice flags to vs. Callers
// must have already verified the actor holds MuteDeafen on the target's channel.
func applyAdminVoiceState(vs *mumble.VoiceState, us *messages.UserState) {
	mute, muteSet := us.Mute, us.Has(messages.UserStateSetMute)

	if us.Has(messages.UserStateSetDeaf) {
		vs.Deaf = us.Deaf
		if vs.Deaf {
			mute, muteSet = true, true
		}
	}
	if muteSet {
		vs.Mute = mute
		if !vs.Mute {
			vs.Deaf = false
		}
	}

	if us.Has(messages.UserStateSetSuppress) {
		vs.Suppress = us.Suppress
	}
	if us.Has(messages.UserStateSetPrioritySpeaker) {
		vs.PrioritySpeaker = us.PrioritySpeaker
	}
}
