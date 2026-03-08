package acl

import (
	"sync"

	"github.com/dchote/go-mumble-server/internal/channel"
	"github.com/dchote/go-mumble-server/internal/database/models"
	"github.com/dchote/go-mumble-server/internal/user"
	"github.com/dchote/go-mumble-server/pkg/mumble"
	"gorm.io/gorm"
)

// Evaluator resolves permissions for a user in a channel using DB ACLs and groups.
type Evaluator struct {
	mu     sync.RWMutex
	db     *gorm.DB
	chans  *channel.Manager
	users  *user.Manager
	cache  map[cacheKey]uint32
}

type cacheKey struct {
	userID    uint32
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

// Check returns whether the user has the given permission in the channel.
func (e *Evaluator) Check(userID uint32, channelID uint32, perm mumble.Permission) bool {
	perms := e.EffectivePermissions(userID, channelID)
	if perms&uint32(mumble.PermissionWrite) != 0 {
		return true
	}
	return perms&uint32(perm) == uint32(perm)
}

// EffectivePermissions returns the permission bitmask for userID in channelID.
func (e *Evaluator) EffectivePermissions(userID uint32, channelID uint32) uint32 {
	key := cacheKey{userID, channelID}
	e.mu.RLock()
	if p, ok := e.cache[key]; ok {
		e.mu.RUnlock()
		return p
	}
	e.mu.RUnlock()

	perms := e.evaluate(userID, channelID)

	e.mu.Lock()
	e.cache[key] = perms
	e.mu.Unlock()
	return perms
}

func (e *Evaluator) evaluate(userID uint32, channelID uint32) uint32 {
	// SuperUser (user ID 0) always has Write
	if userID == 0 {
		return uint32(mumble.PermissionWrite)
	}

	// Build ACL chain from root to target channel
	chain := e.buildACLChain(channelID)
	if len(chain) == 0 {
		return e.defaultRootPerms(userID)
	}

	// Get user for meta groups
	u := e.getUserByUserID(userID)
	userChannelID := uint32(0)
	if u != nil {
		userChannelID = u.ChannelID
	}

	var granted, denied uint32
	for _, entry := range chain {
		if !e.entryApplies(entry, channelID) {
			continue
		}
		if !e.entryMatches(entry, userID, channelID, userChannelID, u) {
			continue
		}
		granted |= entry.Grant
		granted &^= entry.Deny
		denied |= entry.Deny
		denied &^= entry.Grant
	}

	result := granted &^ denied
	// SuperUser override
	if userID == 0 {
		result |= uint32(mumble.PermissionWrite)
	}
	return result
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
	// chain[0] = target, chain[len-1] = root
	// Walk from root to target, collect ACLs where InheritACL allows
	serverID := e.chans.ServerID()
	var entries []aclEntry
	for i := len(chain) - 1; i >= 0; i-- {
		cid := chain[i]
		_, inherit, ok := e.chans.GetChannelWithMeta(cid)
		if !ok {
			continue
		}
		// Include this channel's ACLs if: we're at target, or inherit is true
		include := (uint32(cid) == targetChannelID) || inherit
		if !include {
			continue
		}
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
		// If this channel does not inherit, stop
		if uint32(cid) == targetChannelID && !inherit {
			break
		}
		if !inherit && uint32(cid) != targetChannelID {
			break
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
		if userID == 0 {
			return true // SuperUser is always in admin
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

func (e *Evaluator) getUserByUserID(userID uint32) *mumble.User {
	for _, u := range e.users.ListAll() {
		if u.UserID == userID {
			return u
		}
	}
	return nil
}

func (e *Evaluator) defaultRootPerms(userID uint32) uint32 {
	// Fallback when no ACLs: match default root ACLs
	var p uint32
	// all: Traverse, Enter
	p |= uint32(mumble.PermissionTraverse | mumble.PermissionEnter)
	// auth: Speak, TextMessage, MakeTempChannel, SelfRegister
	if userID > 0 {
		p |= uint32(mumble.PermissionSpeak | mumble.PermissionTextMessage | mumble.PermissionMakeTempChannel | mumble.PermissionSelfRegister)
	}
	return p
}
