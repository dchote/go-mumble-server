package acl

import (
	"github.com/dchote/go-mumble-server/internal/database/models"
	"gorm.io/gorm"
)

// RBAC constants for API user ID mapping.
// API users (from the management API users table) receive synthetic Mumble userIDs
// in a reserved range to avoid collision with registered_users (1, 2, 3...) and
// SuperUser (0). This allows the ACL evaluator to resolve API roles for @admin.
const (
	// APIUserIDBase is the high bit; API userIDs are 0x80000000 | users.id.
	// Max users.id is typically small, so 0x80000001, 0x80000002, etc.
	APIUserIDBase uint32 = 0x80000000
	// APIUserIDMask extracts the DB id from a synthetic userID.
	APIUserIDMask uint32 = 0x7FFFFFFF
)

// IsAPIUserID returns true if userID is in the API user range.
func IsAPIUserID(userID uint32) bool {
	return userID >= APIUserIDBase
}

// MakeAPIUserID creates a synthetic Mumble userID for an API user (users.id).
func MakeAPIUserID(apiUserDBID uint) uint32 {
	if apiUserDBID == 0 {
		return 0
	}
	return APIUserIDBase | uint32(apiUserDBID&uint(APIUserIDMask))
}

// APIUserIDToDBID extracts the users.id from a synthetic userID. Returns 0 if not an API userID.
func APIUserIDToDBID(userID uint32) uint {
	if !IsAPIUserID(userID) {
		return 0
	}
	return uint(userID & APIUserIDMask)
}

// IsAdminForDisplay returns true if the user has admin privileges (API admin or stored admin group).
// Used by the REST API when returning connected users for display in the channel tree.
// Note: userID 0 (server-password users not in registered_users) are not shown as admin.
func IsAdminForDisplay(db *gorm.DB, serverID uint, userID uint32) bool {
	if userID == 0 {
		return false
	}
	if IsAPIUserID(userID) {
		return ResolveAPIAdmin(db, userID)
	}
	return userInStoredAdminGroup(db, serverID, userID)
}

func userInStoredAdminGroup(db *gorm.DB, serverID uint, userID uint32) bool {
	var groups []models.ChannelGroup
	if err := db.Where("server_id = ? AND channel_id = 0 AND name = ?", serverID, "admin").Find(&groups).Error; err != nil || len(groups) == 0 {
		return false
	}
	g := groups[0]
	for _, id := range g.AddUserIDs {
		if id == userID {
			return true
		}
	}
	for _, id := range g.RemoveUserIDs {
		if id == userID {
			return false
		}
	}
	return false
}

// ResolveAPIAdmin returns true if the userID belongs to an API user with role=admin.
// Used by the ACL evaluator for @admin when userID is in the API user range.
func ResolveAPIAdmin(db *gorm.DB, userID uint32) bool {
	dbID := APIUserIDToDBID(userID)
	if dbID == 0 {
		return false
	}
	var u models.User
	err := db.Model(&models.User{}).Where("id = ?", dbID).Select("role").First(&u).Error
	if err != nil {
		return false
	}
	return u.Role == models.RoleAdmin
}
