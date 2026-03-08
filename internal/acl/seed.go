package acl

import (
	"github.com/dchote/go-mumble-server/internal/database/models"
	"github.com/dchote/go-mumble-server/pkg/mumble"
	"gorm.io/gorm"
)

// EnsureDefaultRootACLs seeds default ACLs and groups for the root channel if none exist.
func EnsureDefaultRootACLs(db *gorm.DB, serverID uint) error {
	var count int64
	if err := db.Model(&models.ChannelACL{}).Where("server_id = ? AND channel_id = 0", serverID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	// Create default groups
	groups := []models.ChannelGroup{
		{ServerID: serverID, ChannelID: 0, Name: "admin", Inherit: true, Inheritable: true},
	}
	for _, g := range groups {
		if err := db.Create(&g).Error; err != nil {
			return err
		}
	}
	// Default ACLs: all, auth, admin
	acls := []models.ChannelACL{
		{Priority: 1, ApplyHere: true, ApplySubs: true, GroupName: "all",
			Grant: uint32(mumble.PermissionTraverse | mumble.PermissionEnter)},
		{Priority: 2, ApplyHere: true, ApplySubs: true, GroupName: "auth",
			Grant: uint32(mumble.PermissionSpeak | mumble.PermissionTextMessage | mumble.PermissionMakeTempChannel | mumble.PermissionSelfRegister)},
		{Priority: 3, ApplyHere: true, ApplySubs: true, GroupName: "admin",
			Grant: uint32(mumble.PermissionWrite)},
	}
	for _, a := range acls {
		a.ServerID = serverID
		a.ChannelID = 0
		if err := db.Create(&a).Error; err != nil {
			return err
		}
	}
	return nil
}
