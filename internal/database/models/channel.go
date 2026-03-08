package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"gorm.io/gorm"
)

// Uint32Slice for storing []uint32 in SQLite (JSON).
type Uint32Slice []uint32

func (s Uint32Slice) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	return json.Marshal(s)
}

func (s *Uint32Slice) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("invalid type for Uint32Slice")
	}
	return json.Unmarshal(b, s)
}

// Channel represents a Mumble channel in the channel tree.
type Channel struct {
	ID          uint         `gorm:"primaryKey" json:"id"`
	ServerID    uint         `gorm:"index;not null" json:"server_id"`
	ParentID    *uint        `gorm:"index" json:"parent_id"`
	Parent      *Channel     `gorm:"foreignKey:ParentID" json:"-"`
	Children    []Channel    `gorm:"foreignKey:ParentID" json:"-"`
	Name        string       `gorm:"size:255;not null" json:"name"`
	Description string       `gorm:"type:text" json:"description"`
	Position    int32        `gorm:"not null;default:0" json:"position"`
	MaxUsers    uint32       `gorm:"not null;default:0" json:"max_users"`
	IsTemporary bool         `gorm:"not null;default:false" json:"is_temporary"`
	InheritACL  bool         `gorm:"not null;default:true" json:"inherit_acl"`
	Links       Uint32Slice  `gorm:"type:text" json:"links"` // JSON array of channel IDs
	CreatedAt   int64        `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   int64        `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name.
func (Channel) TableName() string {
	return "channels"
}
