package mumble

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/dchote/go-mumble-server/internal/connection"
	"github.com/dchote/go-mumble-server/pkg/mumble"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol/messages"
)

// Murmur refuses every UserState aimed at SuperUser unless SuperUser sent it, and
// this outranks all other checks (Messages.cpp:775-780). Without it, any user
// holding MuteDeafen could silence the server owner.
func TestHandleUserState_SuperUserIsImmuneToOtherAdmins(t *testing.T) {
	srv := newACLTestServer(t)
	root := srv.chans.GetTree()[0]
	grantACL(t, srv, root.ID, mumble.PermissionMuteDeafen)

	admin := &mumble.User{Name: "admin", UserID: 3, ChannelID: root.ID}
	c, clientSide := connectTestUser(t, srv, admin)
	super := &mumble.User{Name: mumble.SuperUserName, UserID: 1, ChannelID: root.ID, IsSuperUser: true}
	addTestUser(t, srv, super)

	payload := marshalUserState(t, &messages.UserState{
		Session:   super.SessionID,
		Mute:      true,
		SetFields: messages.UserStateSetSession | messages.UserStateSetMute,
	})
	if err := srv.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	if denied := readPermissionDenied(t, clientSide); denied.Type != messages.DenySuperUser {
		t.Errorf("deny type = %d, want DenySuperUser (%d)", denied.Type, messages.DenySuperUser)
	}
	if stored, _ := srv.users.Snapshot(super.SessionID); stored.Mute {
		t.Error("SuperUser was muted by another admin")
	}
}

// SuperUser is only immune to *other* people: its own client must still be able to
// mute itself.
func TestHandleUserState_SuperUserMayChangeItself(t *testing.T) {
	srv := newACLTestServer(t)
	root := srv.chans.GetTree()[0]
	super := &mumble.User{Name: mumble.SuperUserName, UserID: 1, ChannelID: root.ID, IsSuperUser: true}
	c, clientSide := connectTestUser(t, srv, super)

	payload := marshalUserState(t, &messages.UserState{
		SelfMute:  true,
		SetFields: messages.UserStateSetSelfMute,
	})
	if err := srv.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	got := readUserState(t, clientSide)
	assertDeltaEcho(t, got, super.SessionID, map[uint32]bool{
		messages.UserStateSetSelfMute: true,
	})
}

// The SuperUser identity is what grants that immunity, so an anonymous connection
// must not be able to claim the name.
func TestHandleAuthenticate_SuperUserNameRequiresAnAccount(t *testing.T) {
	srv := newACLTestServer(t)
	c, clientSide := newAuthenticatingConn(t)

	payload, err := (&messages.Authenticate{Username: mumble.SuperUserName}).Marshal()
	if err != nil {
		t.Fatalf("Marshal Authenticate: %v", err)
	}
	if err := srv.handleAuthenticate(protocol.MessageAuthenticate, payload, c); err != nil {
		t.Fatalf("handleAuthenticate: %v", err)
	}

	msgType, out := readMessage(t, clientSide)
	if msgType != protocol.MessageReject {
		t.Fatalf("message type = %d, want Reject (%d)", msgType, protocol.MessageReject)
	}
	var reject messages.Reject
	if err := reject.Unmarshal(out); err != nil {
		t.Fatalf("Unmarshal Reject: %v", err)
	}
	if reject.Type != messages.RejectWrongUserPW {
		t.Errorf("reject type = %d, want RejectWrongUserPW", reject.Type)
	}
	if _, ok := srv.users.SnapshotByName(mumble.SuperUserName); ok {
		t.Error("unauthenticated client was allowed to occupy the SuperUser name")
	}
}

// A comment belongs to its author. Murmur only lets somebody else touch it with
// ResetUserContent on root, and even then only to clear it (Messages.cpp:890-905).
func TestHandleUserState_CrossUserCommentRequiresResetUserContent(t *testing.T) {
	srv := newACLTestServer(t)
	root := srv.chans.GetTree()[0]
	actor := &mumble.User{Name: "nosy", UserID: 4, ChannelID: root.ID}
	c, clientSide := connectTestUser(t, srv, actor)
	victim := &mumble.User{Name: "author", ChannelID: root.ID, Comment: "mine"}
	addTestUser(t, srv, victim)

	payload := marshalUserState(t, &messages.UserState{
		Session:   victim.SessionID,
		Comment:   "",
		SetFields: messages.UserStateSetSession | messages.UserStateSetComment,
	})
	if err := srv.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}
	if denied := readPermissionDenied(t, clientSide); denied.Type != messages.DenyPermission {
		t.Errorf("deny type = %d, want DenyPermission", denied.Type)
	}
	if stored, _ := srv.users.Snapshot(victim.SessionID); stored.Comment != "mine" {
		t.Errorf("comment = %q, want unchanged without ResetUserContent", stored.Comment)
	}

	// With the permission, clearing works...
	grantACL(t, srv, root.ID, mumble.PermissionResetUser)
	if err := srv.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}
	got := readUserState(t, clientSide)
	if !got.Has(messages.UserStateSetComment) || got.Comment != "" {
		t.Error("echo must carry the cleared comment")
	}
	if stored, _ := srv.users.Snapshot(victim.SessionID); stored.Comment != "" {
		t.Errorf("comment = %q, want cleared", stored.Comment)
	}
}

// ...but even ResetUserContent only permits clearing, never writing words into
// somebody else's profile.
func TestHandleUserState_CrossUserCommentSetIsAlwaysDenied(t *testing.T) {
	srv := newACLTestServer(t)
	root := srv.chans.GetTree()[0]
	grantACL(t, srv, root.ID, mumble.PermissionResetUser)

	actor := &mumble.User{Name: "impostor", UserID: 6, ChannelID: root.ID}
	c, clientSide := connectTestUser(t, srv, actor)
	victim := &mumble.User{Name: "victim2", ChannelID: root.ID}
	addTestUser(t, srv, victim)

	payload := marshalUserState(t, &messages.UserState{
		Session:   victim.SessionID,
		Comment:   "I love this server",
		SetFields: messages.UserStateSetSession | messages.UserStateSetComment,
	})
	if err := srv.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}
	if denied := readPermissionDenied(t, clientSide); denied.Type != messages.DenyTextTooLong {
		t.Errorf("deny type = %d, want DenyTextTooLong", denied.Type)
	}
	if stored, _ := srv.users.Snapshot(victim.SessionID); stored.Comment != "" {
		t.Errorf("comment = %q, want untouched", stored.Comment)
	}
}

// Comments and avatars are stored and rebroadcast, so they are bounded by the same
// limits the server advertises in ServerConfig.
func TestHandleUserState_OversizedCommentAndTextureDenied(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state *messages.UserState
	}{
		{"comment", &messages.UserState{
			Comment:   strings.Repeat("x", 5001),
			SetFields: messages.UserStateSetComment,
		}},
		{"texture", &messages.UserState{
			Texture:   make([]byte, 131073),
			SetFields: messages.UserStateSetTexture,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newACLTestServer(t)
			root := srv.chans.GetTree()[0]
			u := &mumble.User{Name: "verbose-" + tc.name, ChannelID: root.ID}
			c, clientSide := connectTestUser(t, srv, u)

			if err := srv.handleUserState(protocol.MessageUserState, marshalUserState(t, tc.state), c); err != nil {
				t.Fatalf("handleUserState: %v", err)
			}
			if denied := readPermissionDenied(t, clientSide); denied.Type != messages.DenyTextTooLong {
				t.Errorf("deny type = %d, want DenyTextTooLong", denied.Type)
			}
			stored, _ := srv.users.Snapshot(u.SessionID)
			if stored.Comment != "" || len(stored.Texture) != 0 {
				t.Error("oversized content must not be stored")
			}
		})
	}
}

// Murmur requires Move on the destination OR the target's own Enter right, for
// every move including self-moves (Messages.cpp:810-813).
func TestHandleUserState_SelfMoveNeedsEnterOrMoveOnDestination(t *testing.T) {
	srv := newACLTestServer(t)
	root := srv.chans.GetTree()[0]
	dest := srv.chans.Create(root.ID, "closed", "", 0, false, 0)
	if dest == nil {
		t.Fatal("create destination channel")
	}
	denyACL(t, srv, dest.ID, mumble.PermissionEnter)

	u := &mumble.User{Name: "wanderer", ChannelID: root.ID}
	c, clientSide := connectTestUser(t, srv, u)

	payload := marshalUserState(t, &messages.UserState{
		ChannelID: dest.ID,
		SetFields: messages.UserStateSetChannelID,
	})
	if err := srv.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}
	if denied := readPermissionDenied(t, clientSide); denied.ChannelID != dest.ID {
		t.Errorf("deny channel = %d, want destination %d", denied.ChannelID, dest.ID)
	}
	if stored, _ := srv.users.Snapshot(u.SessionID); stored.ChannelID != root.ID {
		t.Error("user moved into a channel it may not enter")
	}

	// Move on the destination is the other way in: an admin may pull themselves
	// into a channel they cannot Enter.
	grantACL(t, srv, dest.ID, mumble.PermissionMove)
	if err := srv.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}
	if stored, _ := srv.users.Snapshot(u.SessionID); stored.ChannelID != dest.ID {
		t.Errorf("channel = %d, want %d: Move on the destination should allow the move", stored.ChannelID, dest.ID)
	}
}

// Moving somebody else needs Move where they are AND a right to put them where
// they are going. Holding only the former is not enough.
func TestHandleUserState_AdminMoveNeedsRightOnDestinationToo(t *testing.T) {
	srv := newACLTestServer(t)
	root := srv.chans.GetTree()[0]
	dest := srv.chans.Create(root.ID, "vault", "", 0, false, 0)
	if dest == nil {
		t.Fatal("create destination channel")
	}
	grantACL(t, srv, root.ID, mumble.PermissionMove)
	denyACL(t, srv, dest.ID, mumble.PermissionMove|mumble.PermissionEnter)

	actor := &mumble.User{Name: "mover", UserID: 7, ChannelID: root.ID}
	c, clientSide := connectTestUser(t, srv, actor)
	victim := &mumble.User{Name: "moved", ChannelID: root.ID}
	addTestUser(t, srv, victim)

	payload := marshalUserState(t, &messages.UserState{
		Session:   victim.SessionID,
		ChannelID: dest.ID,
		SetFields: messages.UserStateSetSession | messages.UserStateSetChannelID,
	})
	if err := srv.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}
	if denied := readPermissionDenied(t, clientSide); denied.ChannelID != dest.ID {
		t.Errorf("deny channel = %d, want destination %d", denied.ChannelID, dest.ID)
	}
	if stored, _ := srv.users.Snapshot(victim.SessionID); stored.ChannelID != root.ID {
		t.Error("target was pushed into a channel nobody may enter")
	}
}

// Channel.MaxUsers is honoured on move, with murmur's DenyChannelFull
// (Server.cpp:2450, Messages.cpp:814).
func TestHandleUserState_MoveIntoFullChannelDenied(t *testing.T) {
	srv := newACLTestServer(t)
	root := srv.chans.GetTree()[0]
	dest := srv.chans.Create(root.ID, "duo", "", 0, false, 1)
	if dest == nil {
		t.Fatal("create destination channel")
	}
	occupant := &mumble.User{Name: "occupant", ChannelID: dest.ID}
	addTestUser(t, srv, occupant)

	u := &mumble.User{Name: "latecomer", ChannelID: root.ID}
	c, clientSide := connectTestUser(t, srv, u)

	payload := marshalUserState(t, &messages.UserState{
		ChannelID: dest.ID,
		SetFields: messages.UserStateSetChannelID,
	})
	if err := srv.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}
	if denied := readPermissionDenied(t, clientSide); denied.Type != messages.DenyChannelFull {
		t.Errorf("deny type = %d, want DenyChannelFull (%d)", denied.Type, messages.DenyChannelFull)
	}
	if stored, _ := srv.users.Snapshot(u.SessionID); stored.ChannelID != root.ID {
		t.Error("user entered a full channel")
	}
}

// Anyone can create a temporary channel and grant themselves MuteDeafen inside it,
// so murmur re-checks the permission in the first permanent ancestor before
// honouring a mute (Messages.cpp:815-833).
func TestHandleUserState_MuteInTemporaryChannelChecksPermanentParent(t *testing.T) {
	srv := newACLTestServer(t)
	root := srv.chans.GetTree()[0]
	temp := srv.chans.Create(root.ID, "temp-room", "", 0, true, 0)
	if temp == nil {
		t.Fatal("create temporary channel")
	}
	// MuteDeafen only inside the temporary channel — the escalation murmur blocks.
	grantACL(t, srv, temp.ID, mumble.PermissionMuteDeafen)

	actor := &mumble.User{Name: "temp-admin", UserID: 8, ChannelID: temp.ID}
	c, clientSide := connectTestUser(t, srv, actor)
	victim := &mumble.User{Name: "temp-victim", ChannelID: temp.ID}
	addTestUser(t, srv, victim)

	payload := marshalUserState(t, &messages.UserState{
		Session:   victim.SessionID,
		Mute:      true,
		SetFields: messages.UserStateSetSession | messages.UserStateSetMute,
	})
	if err := srv.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}
	if denied := readPermissionDenied(t, clientSide); denied.Type != messages.DenyTemporaryChannel {
		t.Errorf("deny type = %d, want DenyTemporaryChannel (%d)", denied.Type, messages.DenyTemporaryChannel)
	}
	if stored, _ := srv.users.Snapshot(victim.SessionID); stored.Mute {
		t.Error("mute applied from inside a temporary channel without permanent permission")
	}

	// Granting it in the permanent parent makes the same mute legitimate.
	grantACL(t, srv, root.ID, mumble.PermissionMuteDeafen)
	if err := srv.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}
	_ = readUserState(t, clientSide)
	if stored, _ := srv.users.Snapshot(victim.SessionID); !stored.Mute {
		t.Error("mute should apply once the permanent parent grants MuteDeafen")
	}
}

// A server with recording disabled disconnects a client that starts recording,
// rather than silently relaying it (Messages.cpp:1057-1070).
func TestHandleUserState_RecordingDisconnectsWhenNotAllowed(t *testing.T) {
	u := &mumble.User{Name: "taper"}
	s, c, clientSide := newUserStateTestServer(t, u)
	s.SetContentPolicy(false, s.maxTextLength(), s.maxImageLength())

	payload := marshalUserState(t, &messages.UserState{
		Recording: true,
		SetFields: messages.UserStateSetRecording,
	})
	// The kick waits for the UserRemove to reach the socket before closing, so
	// the read has to be in flight while the handler runs.
	handled := make(chan error, 1)
	go func() { handled <- s.handleUserState(protocol.MessageUserState, payload, c) }()

	msgType, out := readMessage(t, clientSide)
	if err := <-handled; err != nil {
		t.Fatalf("handleUserState: %v", err)
	}
	if msgType != protocol.MessageUserRemove {
		t.Fatalf("message type = %d, want UserRemove (%d)", msgType, protocol.MessageUserRemove)
	}
	var removal messages.UserRemove
	if err := removal.Unmarshal(out); err != nil {
		t.Fatalf("Unmarshal UserRemove: %v", err)
	}
	if removal.Reason == "" {
		t.Error("UserRemove must explain why the client was disconnected")
	}
	if stored, _ := s.users.Snapshot(u.SessionID); stored.Recording {
		t.Error("recording must not be applied on a server that forbids it")
	}
}

// Stopping a recording is always allowed, even where starting one is not.
func TestHandleUserState_StoppingRecordingAllowedWhenRecordingForbidden(t *testing.T) {
	u := &mumble.User{Name: "taper2", VoiceState: mumble.VoiceState{Recording: true}}
	s, c, clientSide := newUserStateTestServer(t, u)
	s.SetContentPolicy(false, s.maxTextLength(), s.maxImageLength())

	payload := marshalUserState(t, &messages.UserState{
		SetFields: messages.UserStateSetRecording, // recording=false
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	got := readUserState(t, clientSide)
	if !got.Has(messages.UserStateSetRecording) || got.Recording {
		t.Error("echo must carry recording=false")
	}
}

// Every accepted UserState fans out to every client, so murmur rate-limits the
// self-targeted ones (Messages.cpp:790). Past the budget the message is dropped
// silently, exactly as upstream does.
func TestHandleUserState_SelfUpdatesAreRateLimited(t *testing.T) {
	u := &mumble.User{Name: "flooder"}
	s, c, clientSide := newUserStateTestServer(t, u)

	// Writes are queued (writeCh holds 64), so the whole burst can be driven
	// before anything is read back; otherwise the one-second window could roll
	// over mid-test.
	for i := 0; i < maxUserStatesPerSecond+1; i++ {
		state := &messages.UserState{SetFields: messages.UserStateSetSelfMute}
		state.SelfMute = i%2 == 0
		if err := s.handleUserState(protocol.MessageUserState, marshalUserState(t, state), c); err != nil {
			t.Fatalf("handleUserState: %v", err)
		}
	}

	for i := 0; i < maxUserStatesPerSecond; i++ {
		_ = readUserState(t, clientSide)
	}
	expectNoBroadcast(t, clientSide)
}

func TestRateWindow_AllowsUpToLimitPerWindow(t *testing.T) {
	var r rateWindow
	for i := 0; i < 3; i++ {
		if !r.allow(3) {
			t.Fatalf("call %d was rejected inside the budget", i+1)
		}
	}
	if r.allow(3) {
		t.Error("call past the budget must be rejected")
	}
}

// A connecting client lands in root when the configured default channel has no
// room, instead of being wedged into a full channel (Messages.cpp:300-315).
func TestJoinChannelFor_FallsBackToRootWhenDefaultIsFull(t *testing.T) {
	srv := newACLTestServer(t)
	root := srv.chans.GetTree()[0]
	dest := srv.chans.Create(root.ID, "lobby", "", 0, false, 1)
	if dest == nil {
		t.Fatal("create default channel")
	}
	srv.cfg.DefaultChannel = int(dest.ID)

	if got := srv.joinChannelFor(0); got != dest.ID {
		t.Fatalf("joinChannelFor = %d, want the configured default %d", got, dest.ID)
	}
	addTestUser(t, srv, &mumble.User{Name: "occupant", ChannelID: dest.ID})
	if got := srv.joinChannelFor(0); got != root.ID {
		t.Errorf("joinChannelFor = %d, want root %d once the default is full", got, root.ID)
	}
}

// newAuthenticatingConn returns a connection that has not authenticated yet, for
// exercising handleAuthenticate itself.
func newAuthenticatingConn(t *testing.T) (*connection.Conn, net.Conn) {
	t.Helper()
	clientSide, serverSide := net.Pipe()
	c := connection.New(serverSide, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = c.Run(ctx, protocol.HandlerTable{}) }()
	t.Cleanup(func() {
		cancel()
		_ = clientSide.Close()
	})
	return c, clientSide
}
