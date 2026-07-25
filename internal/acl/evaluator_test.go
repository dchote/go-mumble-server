package acl

import (
	"path/filepath"
	"testing"

	"github.com/dchote/go-mumble-server/internal/channel"
	"github.com/dchote/go-mumble-server/internal/database"
	"github.com/dchote/go-mumble-server/internal/database/models"
	"github.com/dchote/go-mumble-server/internal/user"
	"github.com/dchote/go-mumble-server/pkg/mumble"
	"gorm.io/gorm"
)

const testServerID uint = 1

func newTestEvaluator(t *testing.T) (*Evaluator, *channel.Manager, *gorm.DB, *user.Manager) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "acl.sqlite"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	chans := channel.NewManager(db, testServerID)
	users := user.NewManager(db, 10)
	return NewEvaluator(db, chans, users), chans, db, users
}

// seedACL grants perms to everyone in channelID.
func seedACL(t *testing.T, e *Evaluator, db *gorm.DB, channelID uint32, perms mumble.Permission, applySubs bool) {
	t.Helper()
	row := models.ChannelACL{
		ServerID: testServerID, ChannelID: uint(channelID),
		Priority: 1, ApplyHere: true, ApplySubs: applySubs,
		GroupName: "all", Grant: uint32(perms),
	}
	if err := CreateACL(db, &row); err != nil {
		t.Fatalf("seed ACL: %v", err)
	}
	e.InvalidateCache()
}

func mustCreate(t *testing.T, chans *channel.Manager, parent uint32, name string) *mumble.Channel {
	t.Helper()
	ch := chans.Create(parent, name, "", 0, false, 0)
	if ch == nil {
		t.Fatalf("create channel %q under %d", name, parent)
	}
	return ch
}

// Root is where server-wide policy lives (it is what EnsureDefaultRootACLs writes
// to), so an ACL set there has to reach every channel below it.
func TestEvaluator_RootACLsInheritIntoSubchannels(t *testing.T) {
	e, chans, db, _ := newTestEvaluator(t)
	lobby := mustCreate(t, chans, chans.RootID(), "lobby")
	nested := mustCreate(t, chans, lobby.ID, "nested")

	seedACL(t, e, db, chans.RootID(), mumble.PermissionMuteDeafen, true)

	for _, cid := range []uint32{chans.RootID(), lobby.ID, nested.ID} {
		if !e.Check(SubjectForUserID(7), cid, mumble.PermissionMuteDeafen) {
			t.Errorf("channel %d: root grant did not reach it", cid)
		}
	}
}

// ApplySubs is the switch that decides whether an entry escapes its own channel.
func TestEvaluator_EntryWithoutApplySubsStaysInItsChannel(t *testing.T) {
	e, chans, db, _ := newTestEvaluator(t)
	lobby := mustCreate(t, chans, chans.RootID(), "lobby")

	seedACL(t, e, db, chans.RootID(), mumble.PermissionMuteDeafen, false)

	if !e.Check(SubjectForUserID(7), chans.RootID(), mumble.PermissionMuteDeafen) {
		t.Error("grant does not apply in the channel that carries it")
	}
	if e.Check(SubjectForUserID(7), lobby.ID, mumble.PermissionMuteDeafen) {
		t.Error("grant leaked into a subchannel despite ApplySubs=false")
	}
}

// InheritACL=false means "this channel does not take its parent's ACLs". Its own
// entries still count, and channels below it inherit from it as usual.
func TestEvaluator_NonInheritingChannelIgnoresAncestorACLs(t *testing.T) {
	e, chans, db, _ := newTestEvaluator(t)
	private := mustCreate(t, chans, chans.RootID(), "private")
	below := mustCreate(t, chans, private.ID, "below")
	if err := db.Model(&models.Channel{}).Where("id = ?", private.ID).
		Update("inherit_acl", false).Error; err != nil {
		t.Fatalf("clear inherit_acl: %v", err)
	}
	chans.Reload()

	seedACL(t, e, db, chans.RootID(), mumble.PermissionMuteDeafen, true)
	seedACL(t, e, db, private.ID, mumble.PermissionKick, true)

	if e.Check(SubjectForUserID(7), private.ID, mumble.PermissionMuteDeafen) {
		t.Error("non-inheriting channel picked up an ancestor's ACL")
	}
	if !e.Check(SubjectForUserID(7), private.ID, mumble.PermissionKick) {
		t.Error("non-inheriting channel lost its own ACL")
	}
	if !e.Check(SubjectForUserID(7), below.ID, mumble.PermissionKick) {
		t.Error("child of a non-inheriting channel lost the inherited ACL")
	}
	if e.Check(SubjectForUserID(7), below.ID, mumble.PermissionMuteDeafen) {
		t.Error("ancestor ACL reached past the non-inheriting channel")
	}
}

// The baseline grants are the starting point of every evaluation, not a fallback
// used only while the ACL table is empty: adding one unrelated row must not strip
// a registered user of the rights they had a moment earlier.
func TestEvaluator_BaselineSurvivesTheFirstACLRow(t *testing.T) {
	e, chans, db, _ := newTestEvaluator(t)
	const registeredUser = 5

	before := e.EffectivePermissions(SubjectForUserID(registeredUser), chans.RootID())
	seedACL(t, e, db, chans.RootID(), mumble.PermissionMuteDeafen, true)
	after := e.EffectivePermissions(SubjectForUserID(registeredUser), chans.RootID())

	if missing := before &^ after; missing != 0 {
		t.Errorf("permissions %#x were lost when the first ACL row appeared", missing)
	}
	if !e.Check(SubjectForUserID(registeredUser), chans.RootID(), mumble.PermissionSelfRegister) {
		t.Error("registered user lost SelfRegister")
	}
	if !e.Check(SubjectForUserID(registeredUser), chans.RootID(), mumble.PermissionSpeak) {
		t.Error("registered user lost the Speak baseline")
	}
}

// Unregistered users all share user ID 0, so an evaluation keyed on the account
// alone would answer @in/@out for whichever anonymous session happened to be found
// first. Keying on the session gives each guest their own answer.
func TestEvaluator_AnonymousSessionsResolveInAndOutIndependently(t *testing.T) {
	e, chans, db, users := newTestEvaluator(t)
	lobby := mustCreate(t, chans, chans.RootID(), "lobby")

	// Speak in the lobby only for the people actually in it.
	row := models.ChannelACL{
		ServerID: testServerID, ChannelID: uint(lobby.ID),
		Priority: 1, ApplyHere: true, ApplySubs: true,
		GroupName: "out", Deny: uint32(mumble.PermissionSpeak),
	}
	if err := CreateACL(db, &row); err != nil {
		t.Fatalf("seed ACL: %v", err)
	}

	inside, ok := users.Add(mumble.User{Name: "guest-inside", ChannelID: lobby.ID})
	if !ok {
		t.Fatal("add guest-inside")
	}
	outside, ok := users.Add(mumble.User{Name: "guest-outside", ChannelID: chans.RootID()})
	if !ok {
		t.Fatal("add guest-outside")
	}
	if inside.UserID != 0 || outside.UserID != 0 {
		t.Fatalf("expected both guests to be unregistered, got %d and %d", inside.UserID, outside.UserID)
	}

	if !e.Check(SubjectOf(inside), lobby.ID, mumble.PermissionSpeak) {
		t.Error("guest in the lobby was denied Speak by an @out rule")
	}
	if e.Check(SubjectOf(outside), lobby.ID, mumble.PermissionSpeak) {
		t.Error("guest outside the lobby was granted Speak by the other guest's position")
	}
}

// An unknown channel has no chain at all; everyone still gets the baseline rather
// than an empty mask that would lock the server down.
func TestEvaluator_UnknownChannelFallsBackToBaseline(t *testing.T) {
	e, _, _, _ := newTestEvaluator(t)

	if !e.Check(Subject{}, 4242, mumble.PermissionSpeak) {
		t.Error("baseline Speak missing for an unknown channel")
	}
	if e.Check(Subject{}, 4242, mumble.PermissionMuteDeafen) {
		t.Error("unknown channel handed out an administrative permission")
	}
}
