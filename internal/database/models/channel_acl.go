package models

import (
	"gorm.io/gorm"
)

// ChannelACL represents an ACL entry for a channel.
type ChannelACL struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	ServerID    uint           `gorm:"index;not null" json:"server_id"`
	ChannelID   uint           `gorm:"index;not null" json:"channel_id"`
	Priority    int            `gorm:"not null;default:0" json:"priority"`
	ApplyHere   bool           `gorm:"not null;default:true" json:"apply_here"`
	ApplySubs   bool           `gorm:"not null;default:true" json:"apply_subs"`
	UserID      *int32         `json:"user_id,omitempty"`   // nil = use group
	GroupName   string         `gorm:"size:255" json:"group_name"`     // @group, or meta: all, auth, in, out, sub
	AccessToken string         `gorm:"size:255" json:"access_token"`   // @# token groups
	EvalHere    bool           `gorm:"not null;default:false" json:"eval_here"`     // ~ prefix: resolve in ACL-definition channel
	Invert      bool           `gorm:"not null;default:false" json:"invert"`        // ! prefix: invert selector
	Grant       uint32         `gorm:"not null;default:0" json:"grant"`
	Deny        uint32         `gorm:"not null;default:0" json:"deny"`
	CreatedAt   int64          `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   int64          `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name.
func (ChannelACL) TableName() string {
	return "channel_acls"
}
