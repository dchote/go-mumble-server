package acl

import (
	"sync"

	"github.com/dchote/go-mumble-server/internal/channel"
	"github.com/dchote/go-mumble-server/internal/database/models"
	"github.com/dchote/go-mumble-server/internal/user"
	"github.com/dchote/go-mumble-server/pkg/mumble"
	"gorm.io/gorm"
)

// DefaultPermissions is murmur's grant to every user before any ACL is evaluated
// (ChanACL::Perm defaults): ordinary participation is allowed, administration is
// not. Everything that needs the baseline reads it from here.
const DefaultPermissions = mumble.PermissionTraverse | mumble.PermissionEnter |
	mumble.PermissionSpeak | mumble.PermissionWhisper |
	mumble.PermissionTextMessage | mumble.PermissionListen

// Subject identifies whose permissions are being resolved.
//
// A connected client is identified by its session, not by its registered user ID:
// every unregistered user shares user ID 0, while `@in`/`@out` membership and
// token groups depend on the individual session's channel and access tokens.
// SessionID is 0 for callers that are not a connected client, such as the REST API.
type Subject struct {
	SessionID uint32
	UserID    uint32
}

// SubjectOf builds a Subject from a user record.
func SubjectOf(u mumble.User) Subject {
	return Subject{SessionID: u.SessionID, UserID: u.UserID}
}

// SubjectForUserID identifies a registered account with no session of its own.
func SubjectForUserID(userID uint32) Subject {
	return Subject{UserID: userID}
}

// Evaluator resolves permissions for a user in a channel using DB ACLs and groups.
type Evaluator struct {
	mu    sync.RWMutex
	db    *gorm.DB
	chans *channel.Manager
	users *user.Manager
	cache map[cacheKey]uint32
}

type cacheKey struct {
	subject   Subject
	channelID uint32
}

// NewEvaluator creates an ACL evaluator with DB and manager dependencies.
func NewEvaluator(db *gorm.DB, chans *channel.Manager, users *user.Manager) *Evaluator {
	return &Evaluator{
		db:    db,
		chans: chans,
		users: users,
		cache: make(map[cacheKey]uint32),
	}
}

// InvalidateCache clears the permission cache. Call when ACLs, groups, or user state change.
func (e *Evaluator) InvalidateCache() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cache = make(map[cacheKey]uint32)
}

// Check returns whether the subject has the given permission in the channel.
func (e *Evaluator) Check(subject Subject, channelID uint32, perm mumble.Permission) bool {
	perms := e.EffectivePermissions(subject, channelID)
	if perms&uint32(mumble.PermissionWrite) != 0 {
		return true
	}
	return perms&uint32(perm) == uint32(perm)
}

// EffectivePermissions returns the permission bitmask for the subject in channelID.
func (e *Evaluator) EffectivePermissions(subject Subject, channelID uint32) uint32 {
	key := cacheKey{subject, channelID}
	e.mu.RLock()
	if p, ok := e.cache[key]; ok {
		e.mu.RUnlock()
		return p
	}
	e.mu.RUnlock()

	perms := e.evaluate(subject, channelID)

	e.mu.Lock()
	e.cache[key] = perms
	e.mu.Unlock()
	return perms
}

func (e *Evaluator) evaluate(subject Subject, channelID uint32) uint32 {
	// The same starting point whether or not ACLs exist, so that an admin does not
	// lose their standing rights the moment somebody adds the first ACL row.
	def := e.defaultRootPerms(subject.UserID)

	chain := e.buildACLChain(channelID)
	if len(chain) == 0 {
		return def
	}

	u := e.resolveUser(subject)
	userChannelID := uint32(0)
	if u != nil {
		userChannelID = u.ChannelID
	}

	granted := def
	var denied uint32
	for _, entry := range chain {
		if !e.entryApplies(entry, channelID) {
			continue
		}
		if !e.entryMatches(entry, subject.UserID, channelID, userChannelID, u) {
			continue
		}
		granted |= entry.Grant
		granted &^= entry.Deny
		denied |= entry.Deny
		denied &^= entry.Grant
	}

	return granted &^ denied
}

type aclEntry struct {
	ChannelID   uint
	ApplyHere   bool
	ApplySubs   bool
	UserID      *int32
	GroupName   string
	AccessToken string
	EvalHere    bool
	Invert      bool
	Grant       uint32
	Deny        uint32
}

func (e *Evaluator) buildACLChain(targetChannelID uint32) []aclEntry {
	chain := e.chans.AncestorChain(targetChannelID)
	if len(chain) == 0 {
		return nil
	}
	// chain[0] = target, chain[len-1] = root.
	//
	// InheritACL describes whether a channel takes its *parent's* ACLs, so the
	// walk up stops at the first channel that does not inherit: that channel's
	// own ACLs still count, its ancestors' do not (ACL.cpp, hasPermission).
	top := len(chain) - 1
	for i, cid := range chain {
		_, inherit, ok := e.chans.GetChannelWithMeta(cid)
		if !ok || !inherit {
			top = i
			break
		}
	}

	// Entries are collected outermost-first so that the closer channel's grants
	// and denials are applied last and win.
	serverID := e.chans.ServerID()
	var entries []aclEntry
	for i := top; i >= 0; i-- {
		cid := chain[i]
		var rows []models.ChannelACL
		if err := e.db.Where("server_id = ? AND channel_id = ?", serverID, cid).Order("priority, id").Find(&rows).Error; err != nil {
			continue
		}
		for _, r := range rows {
			entries = append(entries, aclEntry{
				ChannelID:   uint(cid),
				ApplyHere:   r.ApplyHere,
				ApplySubs:   r.ApplySubs,
				UserID:      r.UserID,
				GroupName:   r.GroupName,
				AccessToken: r.AccessToken,
				EvalHere:    r.EvalHere,
				Invert:      r.Invert,
				Grant:       r.Grant,
				Deny:        r.Deny,
			})
		}
	}
	return entries
}

func (e *Evaluator) entryApplies(entry aclEntry, targetChannelID uint32) bool {
	if uint32(entry.ChannelID) == targetChannelID {
		return entry.ApplyHere
	}
	return entry.ApplySubs
}

func (e *Evaluator) entryMatches(entry aclEntry, userID uint32, targetChannelID uint32, userChannelID uint32, u *mumble.User) bool {
	// User selector
	if entry.UserID != nil {
		match := uint32(int32(userID)) == uint32(int32(*entry.UserID))
		if entry.Invert {
			match = !match
		}
		return match
	}

	// Group or token selector
	groupName := entry.GroupName
	token := entry.AccessToken

	// Token group (@#)
	if token != "" {
		match := e.userHasToken(u, token)
		if entry.Invert {
			match = !match
		}
		return match
	}

	// Resolve group
	resolveChannelID := targetChannelID
	if entry.EvalHere {
		resolveChannelID = uint32(entry.ChannelID)
	}
	match := e.userInGroup(userID, groupName, resolveChannelID, userChannelID, u)
	if entry.Invert {
		match = !match
	}
	return match
}

func (e *Evaluator) userHasToken(u *mumble.User, token string) bool {
	if u == nil || token == "" {
		return false
	}
	for _, t := range u.AccessTokens {
		if t == token {
			return true
		}
	}
	return false
}

func (e *Evaluator) userInGroup(userID uint32, groupName string, resolveChannelID uint32, userChannelID uint32, u *mumble.User) bool {
	switch groupName {
	case "all":
		return true
	case "auth":
		return userID > 0
	case "in":
		return userChannelID == resolveChannelID
	case "out":
		return userChannelID != resolveChannelID
	case "admin":
		if IsAPIUserID(userID) {
			return ResolveAPIAdmin(e.db, userID) // RBAC: API users with role=admin
		}
		return e.userInStoredGroup(userID, "admin", resolveChannelID)
	case "sub":
		return e.userInSubChannel(userID, resolveChannelID, userChannelID)
	}
	return e.userInStoredGroup(userID, groupName, resolveChannelID)
}

func (e *Evaluator) userInSubChannel(userID uint32, aclChannelID uint32, userChannelID uint32) bool {
	// @~sub: user is in a sub-channel of aclChannelID (default: min depth 1, no max)
	if userChannelID == aclChannelID {
		return false
	}
	chain := e.chans.AncestorChain(userChannelID)
	for _, cid := range chain {
		if cid == aclChannelID {
			return true // userChannelID is a descendant of aclChannelID
		}
	}
	return false
}

func (e *Evaluator) userInStoredGroup(userID uint32, groupName string, channelID uint32) bool {
	serverID := e.chans.ServerID()
	chain := e.chans.AncestorChain(channelID)
	members := make(map[uint32]bool)
	for _, cid := range chain {
		var groups []models.ChannelGroup
		if err := e.db.Where("server_id = ? AND channel_id = ? AND name = ?", serverID, cid, groupName).Find(&groups).Error; err != nil || len(groups) == 0 {
			continue
		}
		g := groups[0]
		if g.Inherit || cid == channelID {
			for _, id := range g.AddUserIDs {
				members[id] = true
			}
			for _, id := range g.RemoveUserIDs {
				delete(members, id)
			}
		}
		if !g.Inheritable {
			break
		}
	}
	return members[userID]
}

// resolveUser finds the record a subject's channel and access tokens come from.
// The session is authoritative when there is one; falling back to the registered
// ID covers callers with no session, such as the REST API. The returned record is
// a private copy owned by this call: the manager never hands out pointers to live
// records (see user.Manager).
func (e *Evaluator) resolveUser(subject Subject) *mumble.User {
	if e.users == nil {
		return nil
	}
	if subject.SessionID != 0 {
		if u, ok := e.users.Snapshot(subject.SessionID); ok {
			return &u
		}
		return nil
	}
	if subject.UserID == 0 {
		return nil
	}
	u, ok := e.users.SnapshotByUserID(subject.UserID)
	if !ok {
		return nil
	}
	return &u
}

func (e *Evaluator) defaultRootPerms(userID uint32) uint32 {
	p := uint32(DefaultPermissions)
	if userID == 0 {
		return p
	}
	// Any identified account, registered or from the management API, may also
	// create temporary channels and register itself.
	p |= uint32(mumble.PermissionMakeTempChannel | mumble.PermissionSelfRegister)
	if IsAPIUserID(userID) && ResolveAPIAdmin(e.db, userID) {
		p |= uint32(mumble.PermissionWrite)
	}
	return p
}
