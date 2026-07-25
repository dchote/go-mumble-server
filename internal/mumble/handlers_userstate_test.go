package mumble

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/dchote/go-mumble-server/internal/acl"
	"github.com/dchote/go-mumble-server/internal/config"
	"github.com/dchote/go-mumble-server/internal/connection"
	"github.com/dchote/go-mumble-server/internal/database"
	"github.com/dchote/go-mumble-server/internal/database/models"
	"github.com/dchote/go-mumble-server/internal/user"
	"github.com/dchote/go-mumble-server/pkg/mumble"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol/messages"
)

// newUserStateTestServer wires up the minimum needed to drive handleUserState and
// observe what actually reaches a client socket.
func newUserStateTestServer(t *testing.T, u *mumble.User) (*Server, *connection.Conn, net.Conn) {
	t.Helper()

	users := user.NewManager(nil, 10)
	stored, ok := users.Add(*u)
	if !ok {
		t.Fatal("could not add user to manager")
	}
	// Hand the caller back the stored record so it can address the session; the
	// manager owns its own copy from here on.
	*u = stored

	clientSide, serverSide := net.Pipe()
	c := connection.New(serverSide, nil, nil)
	c.SetSessionID(u.SessionID)
	c.SetActive()

	s := &Server{
		users: users,
		conns: map[uint32]*connection.Conn{u.SessionID: c},
	}
	// Same content policy NewServer would install from the murmur defaults.
	s.SetContentPolicy(true, 5000, 131072)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = c.Run(ctx, protocol.HandlerTable{}) }()
	t.Cleanup(func() {
		cancel()
		_ = clientSide.Close()
	})

	return s, c, clientSide
}

// newACLTestServer wires up a full Server with a real ACL evaluator backed by a
// temporary sqlite DB, for tests that need actual permission checks (channel moves,
// MuteDeafen, etc.) rather than the bare users/conns literal above.
func newACLTestServer(t *testing.T) *Server {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "userstate-acl.sqlite")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return NewServer(&config.Config{
		MaxUsers:              100,
		AllowRecording:        true,
		MaxTextMessageLength:  5000,
		MaxImageMessageLength: 131072,
	}, db, 1, nil)
}

// connectTestUser adds u to srv and wires up a connected, active client socket for it.
func connectTestUser(t *testing.T, srv *Server, u *mumble.User) (*connection.Conn, net.Conn) {
	t.Helper()
	stored, ok := srv.users.Add(*u)
	if !ok {
		t.Fatalf("could not add user %s", u.Name)
	}
	*u = stored
	clientSide, serverSide := net.Pipe()
	c := connection.New(serverSide, nil, nil)
	c.SetSessionID(u.SessionID)
	c.SetActive()
	srv.RegisterConn(u.SessionID, c)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = c.Run(ctx, protocol.HandlerTable{}) }()
	t.Cleanup(func() {
		cancel()
		_ = clientSide.Close()
	})
	return c, clientSide
}

// addTestUser registers a user with srv without giving it a socket, for victims of
// admin operations that only need to exist. u is updated with its session ID.
func addTestUser(t *testing.T, srv *Server, u *mumble.User) {
	t.Helper()
	stored, ok := srv.users.Add(*u)
	if !ok {
		t.Fatalf("could not add user %s", u.Name)
	}
	*u = stored
}

// grantACL grants perms to everyone in channelID and flushes the permission cache.
func grantACL(t *testing.T, srv *Server, channelID uint32, perms mumble.Permission) {
	t.Helper()
	seedACL(t, srv, models.ChannelACL{ChannelID: uint(channelID), Grant: uint32(perms)})
}

// denyACL denies perms for everyone in channelID and flushes the permission cache.
func denyACL(t *testing.T, srv *Server, channelID uint32, perms mumble.Permission) {
	t.Helper()
	seedACL(t, srv, models.ChannelACL{ChannelID: uint(channelID), Deny: uint32(perms)})
}

// seedACL writes an "everyone, here and below" entry, the shape every test needs.
func seedACL(t *testing.T, srv *Server, row models.ChannelACL) {
	t.Helper()
	row.ServerID = srv.chans.ServerID()
	row.Priority = 1
	row.ApplyHere = true
	row.ApplySubs = true
	row.GroupName = "all"
	if err := acl.CreateACL(srv.db, &row); err != nil {
		t.Fatalf("seed ACL: %v", err)
	}
	srv.acl.InvalidateCache()
}

func readPermissionDenied(t *testing.T, conn net.Conn) *messages.PermissionDenied {
	t.Helper()
	msgType, payload := readMessage(t, conn)
	if msgType != protocol.MessagePermissionDenied {
		t.Fatalf("message type = %d, want PermissionDenied (%d)", msgType, protocol.MessagePermissionDenied)
	}
	var denied messages.PermissionDenied
	if err := denied.Unmarshal(payload); err != nil {
		t.Fatalf("Unmarshal PermissionDenied: %v", err)
	}
	return &denied
}

func marshalUserState(t *testing.T, us *messages.UserState) []byte {
	t.Helper()
	payload, err := us.Marshal()
	if err != nil {
		t.Fatalf("Marshal UserState: %v", err)
	}
	return payload
}

func readMessage(t *testing.T, conn net.Conn) (protocol.MessageType, []byte) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	msgType, payload, err := protocol.ReadPacket(conn)
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	return msgType, payload
}

func readUserState(t *testing.T, conn net.Conn) *messages.UserState {
	t.Helper()
	msgType, payload := readMessage(t, conn)
	if msgType != protocol.MessageUserState {
		t.Fatalf("message type = %d, want UserState (%d)", msgType, protocol.MessageUserState)
	}
	var out messages.UserState
	if err := out.Unmarshal(payload); err != nil {
		t.Fatalf("Unmarshal UserState: %v", err)
	}
	return &out
}

func expectNoBroadcast(t *testing.T, conn net.Conn) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	if _, _, err := protocol.ReadPacket(conn); err == nil {
		t.Fatal("expected no broadcast for no-op UserState")
	}
}

// Regression test for issue #1. A client that is muted and deafened sends an unmute as
// self_mute=false plus self_deaf=false. The resulting broadcast has to carry those
// fields explicitly, otherwise clients that track mute state from the server echo
// (Mumla, Plumble) never learn they were unmuted and stay locked.
func TestHandleUserState_UnmuteIsVisibleOnTheWire(t *testing.T) {
	u := &mumble.User{
		Name:       "alice",
		VoiceState: mumble.VoiceState{SelfMute: true, SelfDeaf: true},
	}
	s, c, clientSide := newUserStateTestServer(t, u)

	payload := marshalUserState(t, &messages.UserState{
		SetFields: messages.UserStateSetSelfMute | messages.UserStateSetSelfDeaf,
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	stored, ok := s.users.Snapshot(u.SessionID)
	if !ok {
		t.Fatal("user disappeared")
	}
	if stored.SelfMute || stored.SelfDeaf {
		t.Errorf("server state SelfMute=%v SelfDeaf=%v, want both false",
			stored.SelfMute, stored.SelfDeaf)
	}

	got := readUserState(t, clientSide)
	if !got.Has(messages.UserStateSetSelfMute) {
		t.Error("broadcast omitted self_mute, so echo-driven clients stay muted")
	}
	if !got.Has(messages.UserStateSetSelfDeaf) {
		t.Error("broadcast omitted self_deaf")
	}
	if got.SelfMute || got.SelfDeaf {
		t.Errorf("broadcast SelfMute=%v SelfDeaf=%v, want both false", got.SelfMute, got.SelfDeaf)
	}
	if got.Session != u.SessionID {
		t.Errorf("broadcast session = %d, want %d", got.Session, u.SessionID)
	}
	if got.Actor != u.SessionID {
		t.Errorf("broadcast actor = %d, want %d", got.Actor, u.SessionID)
	}
}

// Undeafen alone must leave self_mute set on the server, but murmur's cascade never
// synthesises self_mute when un-deafening (only when deafening), so the broadcast
// must carry self_deaf=false without inventing a self_mute presence bit
// (Messages.cpp:979-993).
func TestHandleUserState_UndeafenLeavesMuteExplicitOnWire(t *testing.T) {
	u := &mumble.User{
		Name:       "alice",
		VoiceState: mumble.VoiceState{SelfMute: true, SelfDeaf: true},
	}
	s, c, clientSide := newUserStateTestServer(t, u)

	payload := marshalUserState(t, &messages.UserState{
		SetFields: messages.UserStateSetSelfDeaf, // self_deaf=false
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	stored, _ := s.users.Snapshot(u.SessionID)
	if !stored.SelfMute || stored.SelfDeaf {
		t.Errorf("SelfMute=%v SelfDeaf=%v, want mute=true deaf=false", stored.SelfMute, stored.SelfDeaf)
	}

	got := readUserState(t, clientSide)
	if got.Has(messages.UserStateSetSelfMute) {
		t.Error("undeafen-only must not invent a self_mute presence bit on the wire")
	}
	if !got.Has(messages.UserStateSetSelfDeaf) || got.SelfDeaf {
		t.Error("broadcast must carry explicit self_deaf=false")
	}
}

func TestHandleUserState_DeafenImpliesMute(t *testing.T) {
	u := &mumble.User{Name: "bob"}
	s, c, clientSide := newUserStateTestServer(t, u)

	payload := marshalUserState(t, &messages.UserState{
		SelfDeaf:  true,
		SetFields: messages.UserStateSetSelfDeaf,
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	stored, _ := s.users.Snapshot(u.SessionID)
	if !stored.SelfDeaf || !stored.SelfMute {
		t.Errorf("SelfMute=%v SelfDeaf=%v, want both true", stored.SelfMute, stored.SelfDeaf)
	}

	got := readUserState(t, clientSide)
	if !got.SelfMute {
		t.Error("broadcast did not report the implied self_mute")
	}
}

// A mute toggle that does not request a channel move must leave the stored channel
// untouched, and — since the broadcast is a delta echo, not a snapshot — must not
// invent a channel_id the client never sent.
func TestHandleUserState_MuteToggleKeepsChannel(t *testing.T) {
	u := &mumble.User{Name: "carol", ChannelID: 7}
	s, c, clientSide := newUserStateTestServer(t, u)

	payload := marshalUserState(t, &messages.UserState{
		SelfMute:  true,
		SetFields: messages.UserStateSetSelfMute,
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	if stored, _ := s.users.Snapshot(u.SessionID); stored.ChannelID != 7 {
		t.Errorf("stored ChannelID = %d, want 7", stored.ChannelID)
	}
	got := readUserState(t, clientSide)
	if got.Has(messages.UserStateSetChannelID) {
		t.Error("broadcast must not carry channel_id when no move was requested")
	}
}

// Regression test for issue #1's regression report: a single-field mute toggle echo
// must carry only session, actor, self_mute — no unrelated voice flags at all.
func TestHandleUserState_MuteToggleEchoCarriesNoUnrelatedVoiceFlags(t *testing.T) {
	u := &mumble.User{Name: "dana", ChannelID: 7}
	s, c, clientSide := newUserStateTestServer(t, u)

	payload := marshalUserState(t, &messages.UserState{
		SelfMute:  true,
		SetFields: messages.UserStateSetSelfMute,
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	got := readUserState(t, clientSide)
	assertDeltaEcho(t, got, u.SessionID, map[uint32]bool{
		messages.UserStateSetSelfMute: true,
	})
}

// Humla/Mumla always send BOTH self_mute and self_deaf on every mute/deafen toggle
// (HumlaService.setSelfMuteDeafState). The echo must carry exactly those two voice
// flags — never admin mute/deaf/suppress/recording/priority_speaker — which is what
// produced the official-client spam in issue #1's regression comment when v0.1.4
// over-broadcast every voice flag as an explicit false.
func TestHandleUserState_HumlaMuteToggleEchoCarriesBothSelfFlagsOnly(t *testing.T) {
	u := &mumble.User{Name: "humla", ChannelID: 7}
	s, c, clientSide := newUserStateTestServer(t, u)

	// Mumla mute button: muted=true, deafened &= muted → self_mute=true, self_deaf=false.
	payload := marshalUserState(t, &messages.UserState{
		SelfMute:  true,
		SelfDeaf:  false,
		SetFields: messages.UserStateSetSelfMute | messages.UserStateSetSelfDeaf,
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	got := readUserState(t, clientSide)
	assertDeltaEcho(t, got, u.SessionID, map[uint32]bool{
		messages.UserStateSetSelfMute: true,
		messages.UserStateSetSelfDeaf: false,
	})
}

// Deafening sends only self_deaf=true; murmur synthesises self_mute=true into the
// same message before echoing it (Messages.cpp:979-985), so a client that tracks
// mute purely from the echo still learns it is muted. This is the handler-level
// counterpart to TestApplySelfVoiceState_MutatesMessageWithSynthesisedChanges.
func TestHandleUserState_DeafenEchoCarriesSynthesisedSelfMute(t *testing.T) {
	u := &mumble.User{Name: "deafener"}
	s, c, clientSide := newUserStateTestServer(t, u)

	payload := marshalUserState(t, &messages.UserState{
		SelfDeaf:  true,
		SetFields: messages.UserStateSetSelfDeaf,
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	got := readUserState(t, clientSide)
	assertDeltaEcho(t, got, u.SessionID, map[uint32]bool{
		messages.UserStateSetSelfDeaf: true,
		messages.UserStateSetSelfMute: true,
	})
	stored, _ := s.users.Snapshot(u.SessionID)
	if !stored.SelfMute || !stored.SelfDeaf {
		t.Errorf("stored SelfMute=%v SelfDeaf=%v, want both true", stored.SelfMute, stored.SelfDeaf)
	}
}

// The inverse cascade: unmuting clears deafen, and the echo must say so explicitly
// (Messages.cpp:987-993). This is the exact path issue #1 reported as broken.
func TestHandleUserState_UnmuteEchoCarriesSynthesisedSelfDeafFalse(t *testing.T) {
	u := &mumble.User{
		Name:       "unmuter",
		VoiceState: mumble.VoiceState{SelfMute: true, SelfDeaf: true},
	}
	s, c, clientSide := newUserStateTestServer(t, u)

	payload := marshalUserState(t, &messages.UserState{
		SetFields: messages.UserStateSetSelfMute, // self_mute=false
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	got := readUserState(t, clientSide)
	assertDeltaEcho(t, got, u.SessionID, map[uint32]bool{
		messages.UserStateSetSelfMute: false,
		messages.UserStateSetSelfDeaf: false,
	})
	stored, _ := s.users.Snapshot(u.SessionID)
	if stored.SelfMute || stored.SelfDeaf {
		t.Errorf("stored SelfMute=%v SelfDeaf=%v, want both false", stored.SelfMute, stored.SelfDeaf)
	}
}

// The administrative cascade has the same shape: deaf=true synthesises mute=true
// into the echoed message (Messages.cpp:1023-1029).
func TestHandleUserState_AdminDeafEchoCarriesSynthesisedMute(t *testing.T) {
	srv := newACLTestServer(t)
	root := srv.chans.GetTree()[0]
	admin := &mumble.User{Name: "root-admin", UserID: 5, ChannelID: root.ID}
	grantACL(t, srv, root.ID, mumble.PermissionMuteDeafen)

	c, clientSide := connectTestUser(t, srv, admin)
	victim := &mumble.User{Name: "deafened-victim", ChannelID: root.ID}
	addTestUser(t, srv, victim)

	payload := marshalUserState(t, &messages.UserState{
		Session:   victim.SessionID,
		Deaf:      true,
		SetFields: messages.UserStateSetSession | messages.UserStateSetDeaf,
	})
	if err := srv.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	got := readUserState(t, clientSide)
	if got.Session != victim.SessionID || got.Actor != admin.SessionID {
		t.Errorf("echo session/actor = %d/%d, want %d/%d", got.Session, got.Actor, victim.SessionID, admin.SessionID)
	}
	if !got.Has(messages.UserStateSetDeaf) || !got.Deaf {
		t.Error("echo must carry deaf=true")
	}
	if !got.Has(messages.UserStateSetMute) || !got.Mute {
		t.Error("echo must carry the synthesised mute=true that deafening implies")
	}
	stored, _ := srv.users.Snapshot(victim.SessionID)
	if !stored.Mute || !stored.Deaf {
		t.Errorf("stored Mute=%v Deaf=%v, want both true", stored.Mute, stored.Deaf)
	}
}

// deltaEchoFields enumerates every field a delta echo could carry beyond
// session/actor, so a test can assert on all of them at once.
func deltaEchoFields(us *messages.UserState) []struct {
	name  string
	bit   uint32
	value bool
} {
	return []struct {
		name  string
		bit   uint32
		value bool
	}{
		{"mute", messages.UserStateSetMute, us.Mute},
		{"deaf", messages.UserStateSetDeaf, us.Deaf},
		{"suppress", messages.UserStateSetSuppress, us.Suppress},
		{"self_mute", messages.UserStateSetSelfMute, us.SelfMute},
		{"self_deaf", messages.UserStateSetSelfDeaf, us.SelfDeaf},
		{"priority_speaker", messages.UserStateSetPrioritySpeaker, us.PrioritySpeaker},
		{"recording", messages.UserStateSetRecording, us.Recording},
		{"channel_id", messages.UserStateSetChannelID, us.ChannelID != 0},
		{"name", messages.UserStateSetName, us.Name != ""},
		{"comment", messages.UserStateSetComment, us.Comment != ""},
	}
}

// assertDeltaEcho checks that a delta echo carries session and actor, that every
// field in want is present with exactly the value given (so an explicit false is
// distinguishable from an omitted field), and that no other field is present at all.
func assertDeltaEcho(t *testing.T, got *messages.UserState, session uint32, want map[uint32]bool) {
	t.Helper()
	if got.Session != session || !got.Has(messages.UserStateSetSession) {
		t.Error("broadcast must carry session")
	}
	if got.Actor != session || !got.Has(messages.UserStateSetActor) {
		t.Error("broadcast must carry actor")
	}
	for _, f := range deltaEchoFields(got) {
		wantValue, wanted := want[f.bit]
		switch {
		case wanted && !got.Has(f.bit):
			t.Errorf("echo is missing %s, which the cascade should have set to %v", f.name, wantValue)
		case wanted && f.value != wantValue:
			t.Errorf("echo has %s = %v, want %v", f.name, f.value, wantValue)
		case !wanted && got.Has(f.bit):
			t.Errorf("echo must not carry unrelated field %s", f.name)
		}
	}
}

func TestHandleUserState_SelfFieldsRejectedForOtherUsers(t *testing.T) {
	actor := &mumble.User{Name: "dave"}
	s, c, clientSide := newUserStateTestServer(t, actor)

	victim := &mumble.User{Name: "erin"}
	if stored, ok := s.users.Add(*victim); ok {
		*victim = stored
	} else {
		t.Fatal("could not add victim")
	}

	payload := marshalUserState(t, &messages.UserState{
		Session:   victim.SessionID,
		SelfMute:  true,
		SetFields: messages.UserStateSetSession | messages.UserStateSetSelfMute,
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	if stored, _ := s.users.Snapshot(victim.SessionID); stored.SelfMute {
		t.Error("one user managed to set another user's self_mute")
	}
	expectNoBroadcast(t, clientSide)
}

func TestHandleUserState_EmptyMessageDoesNotBroadcast(t *testing.T) {
	u := &mumble.User{Name: "empty"}
	s, c, clientSide := newUserStateTestServer(t, u)

	if err := s.handleUserState(protocol.MessageUserState, nil, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}
	expectNoBroadcast(t, clientSide)
}

func TestHandleUserState_SuppressAssertDenied(t *testing.T) {
	u := &mumble.User{Name: "frank"}
	s, c, clientSide := newUserStateTestServer(t, u)

	payload := marshalUserState(t, &messages.UserState{
		Suppress:  true,
		SetFields: messages.UserStateSetSuppress,
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	msgType, payloadOut := readMessage(t, clientSide)
	if msgType != protocol.MessagePermissionDenied {
		t.Fatalf("message type = %d, want PermissionDenied", msgType)
	}
	var denied messages.PermissionDenied
	if err := denied.Unmarshal(payloadOut); err != nil {
		t.Fatalf("Unmarshal PermissionDenied: %v", err)
	}
	if denied.Type != messages.DenyPermission {
		t.Errorf("deny type = %d, want DenyPermission", denied.Type)
	}
	if stored, _ := s.users.Snapshot(u.SessionID); stored.Suppress {
		t.Error("suppress assert should not have been applied")
	}
}

func TestMuteSession_UnmuteClearsDeaf(t *testing.T) {
	u := &mumble.User{
		Name:       "grace",
		VoiceState: mumble.VoiceState{Mute: true, Deaf: true},
	}
	s, _, clientSide := newUserStateTestServer(t, u)

	if !s.MuteSession(u.SessionID, false) {
		t.Fatal("MuteSession returned false")
	}
	stored, _ := s.users.Snapshot(u.SessionID)
	if stored.Mute || stored.Deaf {
		t.Errorf("Mute=%v Deaf=%v, want both false", stored.Mute, stored.Deaf)
	}
	got := readUserState(t, clientSide)
	if !got.Has(messages.UserStateSetMute) || got.Mute {
		t.Error("broadcast must carry explicit mute=false")
	}
	if !got.Has(messages.UserStateSetDeaf) || got.Deaf {
		t.Error("broadcast must carry explicit deaf=false after unmute")
	}
}

func TestHandleUserState_AdminMuteRequiresMuteDeafen(t *testing.T) {
	srv := newACLTestServer(t)

	actor := &mumble.User{Name: "adminactor", UserID: 2}
	c, clientSide := connectTestUser(t, srv, actor)
	victim := &mumble.User{Name: "victim"}
	if stored, ok := srv.users.Add(*victim); ok {
		*victim = stored
	} else {
		t.Fatal("add victim")
	}

	payload := marshalUserState(t, &messages.UserState{
		Session:   victim.SessionID,
		Mute:      true,
		SetFields: messages.UserStateSetSession | messages.UserStateSetMute,
	})
	if err := srv.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	msgType, _ := readMessage(t, clientSide)
	if msgType != protocol.MessagePermissionDenied {
		t.Fatalf("message type = %d, want PermissionDenied (actor lacks MuteDeafen)", msgType)
	}
	if stored, _ := srv.users.Snapshot(victim.SessionID); stored.Mute {
		t.Error("mute should not apply without MuteDeafen")
	}
}

func TestHandleUserState_RecordingAnnounces(t *testing.T) {
	u := &mumble.User{Name: "recorder"}
	s, c, clientSide := newUserStateTestServer(t, u)

	payload := marshalUserState(t, &messages.UserState{
		Recording: true,
		SetFields: messages.UserStateSetRecording,
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	// First message is UserState, second is the root-tree TextMessage announcement.
	_ = readUserState(t, clientSide)
	msgType, payloadOut := readMessage(t, clientSide)
	if msgType != protocol.MessageTextMessage {
		t.Fatalf("message type = %d, want TextMessage", msgType)
	}
	var tm messages.TextMessage
	if err := tm.Unmarshal(payloadOut); err != nil {
		t.Fatalf("Unmarshal TextMessage: %v", err)
	}
	if tm.Message == "" || len(tm.TreeID) == 0 || tm.TreeID[0] != 0 {
		t.Errorf("unexpected recording announcement: %+v", tm)
	}
}

// Resending the same recording value is a no-op: no state change, no announcement,
// no broadcast at all — matching murmur's `bRecording != msg.recording()` guard
// (Messages.cpp:1048), not just "has_recording() was present".
func TestHandleUserState_RecordingResendSameValueIsNoOp(t *testing.T) {
	u := &mumble.User{Name: "recorder2", VoiceState: mumble.VoiceState{Recording: true}}
	s, c, clientSide := newUserStateTestServer(t, u)

	payload := marshalUserState(t, &messages.UserState{
		Recording: true,
		SetFields: messages.UserStateSetRecording,
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}
	expectNoBroadcast(t, clientSide)
}

// Renaming is never done via UserState; has_name() must be denied unconditionally,
// even when a client targets itself (Messages.cpp:784-787).
func TestHandleUserState_RenameDenied(t *testing.T) {
	u := &mumble.User{Name: "henry"}
	s, c, clientSide := newUserStateTestServer(t, u)

	payload := marshalUserState(t, &messages.UserState{
		Name:      "newname",
		SetFields: messages.UserStateSetName,
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	msgType, payloadOut := readMessage(t, clientSide)
	if msgType != protocol.MessagePermissionDenied {
		t.Fatalf("message type = %d, want PermissionDenied", msgType)
	}
	var denied messages.PermissionDenied
	if err := denied.Unmarshal(payloadOut); err != nil {
		t.Fatalf("Unmarshal PermissionDenied: %v", err)
	}
	if denied.Type != messages.DenyUserName {
		t.Errorf("deny type = %d, want DenyUserName", denied.Type)
	}
	if stored, _ := s.users.Snapshot(u.SessionID); stored.Name != "henry" {
		t.Error("name must not change via UserState")
	}
}

// Requesting the channel you're already in drops the entire message, per murmur's
// plain `return` (Messages.cpp:800-803) — not just the move, everything bundled
// with it too.
func TestHandleUserState_SameChannelDropsWholeMessage(t *testing.T) {
	srv := newACLTestServer(t)
	root := srv.chans.GetTree()[0]
	u := &mumble.User{Name: "ivan", ChannelID: root.ID}
	c, clientSide := connectTestUser(t, srv, u)

	payload := marshalUserState(t, &messages.UserState{
		ChannelID: root.ID,
		SelfMute:  true,
		SetFields: messages.UserStateSetChannelID | messages.UserStateSetSelfMute,
	})
	if err := srv.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}
	expectNoBroadcast(t, clientSide)
	if stored, _ := srv.users.Snapshot(u.SessionID); stored.SelfMute {
		t.Error("self_mute bundled with a same-channel move must not apply either")
	}
}

// A channel move must clear priority speaker and resync suppress from the Speak
// ACL of the destination channel, echoing only the fields that actually flipped
// (Server.cpp:2042-2055), on top of the channel_id delta itself.
func TestHandleUserState_ChannelMoveEchoesFlippedSuppressAndPriority(t *testing.T) {
	srv := newACLTestServer(t)
	root := srv.chans.GetTree()[0]
	dest := srv.chans.Create(root.ID, "no-speak", "", 0, false, 0)
	if dest == nil {
		t.Fatal("create destination channel")
	}
	// Deny Speak in the destination channel so the move flips Suppress to true.
	denyACL(t, srv, dest.ID, mumble.PermissionSpeak)

	u := &mumble.User{Name: "julia", ChannelID: root.ID, VoiceState: mumble.VoiceState{PrioritySpeaker: true}}
	c, clientSide := connectTestUser(t, srv, u)

	payload := marshalUserState(t, &messages.UserState{
		ChannelID: dest.ID,
		SetFields: messages.UserStateSetChannelID,
	})
	if err := srv.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	// Self-moves also receive a PermissionQuery for the destination channel, sent
	// ahead of the UserState broadcast.
	if msgType, _ := readMessage(t, clientSide); msgType != protocol.MessagePermissionQuery {
		t.Fatalf("message type = %d, want PermissionQuery", msgType)
	}

	got := readUserState(t, clientSide)
	if got.ChannelID != dest.ID || !got.Has(messages.UserStateSetChannelID) {
		t.Errorf("broadcast ChannelID = %d, want %d", got.ChannelID, dest.ID)
	}
	if !got.Has(messages.UserStateSetPrioritySpeaker) || got.PrioritySpeaker {
		t.Error("broadcast must carry explicit priority_speaker=false, cleared by the move")
	}
	if !got.Has(messages.UserStateSetSuppress) || !got.Suppress {
		t.Error("broadcast must carry explicit suppress=true, flipped by the destination's Speak ACL")
	}
	stored, _ := srv.users.Snapshot(u.SessionID)
	if stored.PrioritySpeaker || !stored.Suppress {
		t.Errorf("PrioritySpeaker=%v Suppress=%v, want false/true", stored.PrioritySpeaker, stored.Suppress)
	}
}

// RefreshSuppressStates must broadcast a minimal {session, suppress} delta with the
// real value, not upstream murmur's hardcoded suppress=true (Server.cpp:2187-2199).
func TestRefreshSuppressStates_SendsMinimalDeltaWithRealValue(t *testing.T) {
	srv := newACLTestServer(t)
	root := srv.chans.GetTree()[0]
	u := &mumble.User{Name: "karl", ChannelID: root.ID}
	_, clientSide := connectTestUser(t, srv, u)

	// Deny Speak at root for everyone so the refresh flips Suppress to true.
	denyACL(t, srv, root.ID, mumble.PermissionSpeak)

	srv.RefreshSuppressStates()

	got := readUserState(t, clientSide)
	if got.Session != u.SessionID || !got.Has(messages.UserStateSetSession) {
		t.Error("broadcast must carry session")
	}
	if !got.Has(messages.UserStateSetSuppress) || !got.Suppress {
		t.Error("broadcast must carry explicit suppress=true (the real value)")
	}
	if got.Has(messages.UserStateSetChannelID) || got.Has(messages.UserStateSetSelfMute) ||
		got.Has(messages.UserStateSetMute) || got.Has(messages.UserStateSetActor) {
		t.Error("RefreshSuppressStates broadcast must carry nothing beyond session/suppress")
	}
}

// MuteSession must broadcast a minimal {session, mute} delta (no snapshot fields
// like channel_id/name/texture), plus deaf only when the cascade cleared it.
func TestMuteSession_SendsMinimalDelta(t *testing.T) {
	u := &mumble.User{Name: "lena", ChannelID: 7}
	s, _, clientSide := newUserStateTestServer(t, u)

	if !s.MuteSession(u.SessionID, true) {
		t.Fatal("MuteSession returned false")
	}
	got := readUserState(t, clientSide)
	if !got.Has(messages.UserStateSetMute) || !got.Mute {
		t.Error("broadcast must carry explicit mute=true")
	}
	if got.Has(messages.UserStateSetChannelID) || got.Has(messages.UserStateSetDeaf) ||
		got.Has(messages.UserStateSetActor) || got.Has(messages.UserStateSetName) {
		t.Error("MuteSession broadcast must carry nothing beyond session/mute (deaf only if cleared)")
	}
}

// plugin_context/plugin_identity are applied server-side but never set murmur's
// bBroadcast on their own (Messages.cpp:995-1007). A plugin-only UserState must
// produce no broadcast at all — not a bare {session, actor} no-op.
func TestHandleUserState_PluginOnlyDoesNotBroadcast(t *testing.T) {
	u := &mumble.User{Name: "plugin"}
	s, c, clientSide := newUserStateTestServer(t, u)

	payload := marshalUserState(t, &messages.UserState{
		PluginIdentity: "pos-audio-id",
		PluginContext:  []byte{0x01, 0x02},
		SetFields:      messages.UserStateSetPluginIdentity | messages.UserStateSetPluginContext,
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}
	expectNoBroadcast(t, clientSide)

	stored, ok := s.users.Snapshot(u.SessionID)
	if !ok {
		t.Fatal("user disappeared")
	}
	if stored.PluginIdentity != "pos-audio-id" {
		t.Errorf("PluginIdentity = %q, want applied server-side", stored.PluginIdentity)
	}
	if len(stored.PluginContext) != 2 {
		t.Errorf("PluginContext len = %d, want applied server-side", len(stored.PluginContext))
	}
}

// Clients at or above 1.2.3 render recording from the UserState.recording field
// themselves; the legacy TextMessage announcement must not be sent to them
// (Messages.cpp:1075), or they would see a duplicate "started/stopped recording".
func TestHandleUserState_RecordingAnnouncementSkippedForModernClients(t *testing.T) {
	u := &mumble.User{Name: "modern"}
	s, c, clientSide := newUserStateTestServer(t, u)
	c.SetClientVersion(version1_2_3)

	payload := marshalUserState(t, &messages.UserState{
		Recording: true,
		SetFields: messages.UserStateSetRecording,
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	got := readUserState(t, clientSide)
	if !got.Has(messages.UserStateSetRecording) || !got.Recording {
		t.Error("UserState must still carry recording=true")
	}
	expectNoBroadcast(t, clientSide) // no legacy TextMessage for >= 1.2.3
}

// Clients older than 1.2.3 (and clients that never sent Version, ClientVersion==0)
// still receive the legacy TextMessage announcement.
func TestHandleUserState_RecordingAnnouncementSentToLegacyClients(t *testing.T) {
	u := &mumble.User{Name: "legacy"}
	s, c, clientSide := newUserStateTestServer(t, u)
	c.SetClientVersion(version1_2_3 - 1)

	payload := marshalUserState(t, &messages.UserState{
		Recording: true,
		SetFields: messages.UserStateSetRecording,
	})
	if err := s.handleUserState(protocol.MessageUserState, payload, c); err != nil {
		t.Fatalf("handleUserState: %v", err)
	}

	_ = readUserState(t, clientSide)
	msgType, payloadOut := readMessage(t, clientSide)
	if msgType != protocol.MessageTextMessage {
		t.Fatalf("message type = %d, want TextMessage for legacy client", msgType)
	}
	var tm messages.TextMessage
	if err := tm.Unmarshal(payloadOut); err != nil {
		t.Fatalf("Unmarshal TextMessage: %v", err)
	}
	if tm.Message == "" {
		t.Error("legacy recording announcement must be non-empty")
	}
}

// Post-auth join announce (Broadcast(joiner, userToState(*joiner))) must omit every
// false voice flag. This is the exact wire shape that produced the issue #1
// regression spam when v0.1.4 set all seven presence bits on every snapshot:
// observers logged "unmuted / stopped recording / priority speaker / ..." for a
// freshly connected Mumla that had toggled nothing.
func TestJoinAnnounce_UnmutedUserSnapshotOmitsFalseVoiceFlags(t *testing.T) {
	observer := &mumble.User{Name: "observer"}
	s, _, observerSide := newUserStateTestServer(t, observer)

	joiner := &mumble.User{Name: "mumla-joiner"}
	if stored, ok := s.users.Add(*joiner); ok {
		*joiner = stored
	} else {
		t.Fatal("could not add joiner")
	}

	// Mirrors handleAuthenticate's join announce: skip the joining session, send
	// the snapshot to everyone else.
	s.Broadcast(joiner.SessionID, protocol.MessageUserState, userToState(*joiner))

	got := readUserState(t, observerSide)
	if got.Session != joiner.SessionID {
		t.Errorf("session = %d, want joiner %d", got.Session, joiner.SessionID)
	}
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
		if got.Has(tt.bit) {
			t.Errorf("join announce must not carry false voice flag %s (issue #1 regression)", tt.name)
		}
	}
	if !got.Has(messages.UserStateSetSession) || !got.Has(messages.UserStateSetChannelID) {
		t.Error("join snapshot must still carry session and channel_id")
	}
}
