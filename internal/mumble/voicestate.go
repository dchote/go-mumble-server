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
//
// murmur takes its MumbleProto::UserState by non-const reference and mutates it in
// place before rebroadcasting the same message, so a synthesised mute reaches the
// wire alongside the deafen that implied it. These helpers do the same: they mutate
// us (setting both the value and its presence bit) whenever they synthesise a change,
// so the caller can broadcast us directly instead of a full snapshot.

// applySelfVoiceState applies a client's own mute/deafen request to vs, mutating us
// with any change murmur's cascade would have synthesised (Messages.cpp:979-993).
func applySelfVoiceState(vs *mumble.VoiceState, us *messages.UserState) {
	if us.Has(messages.UserStateSetSelfDeaf) {
		vs.SelfDeaf = us.SelfDeaf
		if vs.SelfDeaf {
			vs.SelfMute = true
			us.SelfMute = true
			us.SetFields |= messages.UserStateSetSelfMute
		}
	}
	if us.Has(messages.UserStateSetSelfMute) {
		vs.SelfMute = us.SelfMute
		if !vs.SelfMute {
			vs.SelfDeaf = false
			us.SelfDeaf = false
			us.SetFields |= messages.UserStateSetSelfDeaf
		}
	}

	if us.Has(messages.UserStateSetRecording) {
		vs.Recording = us.Recording
	}
}

// applyAdminVoiceState applies administratively imposed voice flags to vs, mutating us
// with any change murmur's cascade would have synthesised (Messages.cpp:1017-1046).
// Callers must have already verified the actor holds MuteDeafen on the target's channel.
func applyAdminVoiceState(vs *mumble.VoiceState, us *messages.UserState) {
	if us.Has(messages.UserStateSetDeaf) {
		vs.Deaf = us.Deaf
		if vs.Deaf {
			vs.Mute = true
			us.Mute = true
			us.SetFields |= messages.UserStateSetMute
		}
	}
	if us.Has(messages.UserStateSetMute) {
		vs.Mute = us.Mute
		if !vs.Mute {
			vs.Deaf = false
			us.Deaf = false
			us.SetFields |= messages.UserStateSetDeaf
		}
	}

	if us.Has(messages.UserStateSetSuppress) {
		vs.Suppress = us.Suppress
	}
	if us.Has(messages.UserStateSetPrioritySpeaker) {
		vs.PrioritySpeaker = us.PrioritySpeaker
	}
}

// applyChannelEnterEffects mirrors murmur's Server::userEnterChannel side effects that
// fire on every channel move (Server.cpp:2042-2055): priority speaker is always
// cleared, and suppress is resynced from the Speak ACL in the destination channel.
// maySpeak must be resolved by the caller via Server.maySpeak BEFORE taking the
// user manager's lock (see applySuppressFromSpeak); this stays a pure state mutator
// so it is safe to call from inside user.Manager.UpdateUser. It reports which fields
// actually changed so callers echo only real flips, matching murmur.
func applyChannelEnterEffects(u *mumble.User, maySpeak bool) (prioritySpeakerChanged, suppressChanged bool) {
	if u.PrioritySpeaker {
		u.PrioritySpeaker = false
		prioritySpeakerChanged = true
	}
	suppressChanged = applySuppressFromSpeak(u, maySpeak)
	return prioritySpeakerChanged, suppressChanged
}

// applySuppressFromSpeak mirrors murmur's userEnterChannel: Suppress tracks the
// inverse of Speak permission so clients that key off the suppress flag stay in
// sync. maySpeak must be resolved by the caller (via the ACL evaluator) before
// calling this: the evaluator can itself enumerate connected users (for group
// membership), so resolving it from inside user.Manager.UpdateUser's callback — which
// already holds the manager's lock — would deadlock against that same lock. Returns
// whether Suppress actually flipped, so callers can echo only real changes.
func applySuppressFromSpeak(u *mumble.User, maySpeak bool) bool {
	if maySpeak == u.Suppress {
		u.Suppress = !maySpeak
		return true
	}
	return false
}
