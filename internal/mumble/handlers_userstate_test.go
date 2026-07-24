package mumble

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/dchote/go-mumble-server/internal/config"
	"github.com/dchote/go-mumble-server/internal/connection"
	"github.com/dchote/go-mumble-server/internal/database"
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
	if !users.Add(u) {
		t.Fatal("could not add user to manager")
	}

	clientSide, serverSide := net.Pipe()
	c := connection.New(serverSide, nil, nil)
	c.SetSessionID(u.SessionID)
	c.SetActive()

	s := &Server{
		users: users,
		conns: map[uint32]*connection.Conn{u.SessionID: c},
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = c.Run(ctx, protocol.HandlerTable{}) }()
	t.Cleanup(func() {
		cancel()
		_ = clientSide.Close()
	})

	return s, c, clientSide
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

// Undeafen alone must leave self_mute set, and the broadcast must still carry
// self_mute=true explicitly so echo-driven clients do not invent an unmute.
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
	if !got.Has(messages.UserStateSetSelfMute) || !got.SelfMute {
		t.Error("broadcast must carry explicit self_mute=true after undeafen-only")
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
	if got := readUserState(t, clientSide); got.ChannelID != 7 {
		t.Errorf("broadcast ChannelID = %d, want 7", got.ChannelID)
	}
}

func TestHandleUserState_SelfFieldsRejectedForOtherUsers(t *testing.T) {
	actor := &mumble.User{Name: "dave"}
	s, c, clientSide := newUserStateTestServer(t, actor)

	victim := &mumble.User{Name: "erin"}
	if !s.users.Add(victim) {
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
	dbPath := filepath.Join(t.TempDir(), "userstate-acl.sqlite")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	srv := NewServer(&config.Config{MaxUsers: 100}, db, 1, nil)

	actor := &mumble.User{Name: "adminactor", UserID: 2}
	if !srv.users.Add(actor) {
		t.Fatal("add actor")
	}
	victim := &mumble.User{Name: "victim"}
	if !srv.users.Add(victim) {
		t.Fatal("add victim")
	}

	clientSide, serverSide := net.Pipe()
	c := connection.New(serverSide, nil, nil)
	c.SetSessionID(actor.SessionID)
	c.SetActive()
	srv.RegisterConn(actor.SessionID, c)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = c.Run(ctx, protocol.HandlerTable{}) }()
	t.Cleanup(func() {
		cancel()
		_ = clientSide.Close()
	})

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
