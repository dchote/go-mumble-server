package models

import (
	"gorm.io/gorm"
)

// ChannelGroup represents a group on a channel.
type ChannelGroup struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	ServerID      uint           `gorm:"index;not null" json:"server_id"`
	ChannelID     uint           `gorm:"index;not null" json:"channel_id"`
	Name          string         `gorm:"size:255;not null" json:"name"`
	Inherit       bool           `gorm:"not null;default:true" json:"inherit"`
	Inheritable   bool           `gorm:"not null;default:true" json:"inheritable"`
	AddUserIDs    Uint32Slice    `gorm:"type:text" json:"add_user_ids"`
	RemoveUserIDs Uint32Slice    `gorm:"type:text" json:"remove_user_ids"`
	CreatedAt     int64          `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     int64          `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name.
func (ChannelGroup) TableName() string {
	return "channel_groups"
}
