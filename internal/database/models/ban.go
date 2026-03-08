package models

import (
	"time"

	"gorm.io/gorm"
)

// Ban represents a server ban (IP or certificate hash).
type Ban struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	ServerID   uint           `gorm:"index;not null" json:"server_id"`
	Address    []byte         `gorm:"size:16" json:"-"` // IPv4 (4) or IPv6 (16)
	Mask       uint32         `gorm:"not null;default:32" json:"mask"`
	Name       string         `gorm:"size:255" json:"name"`
	Hash       string         `gorm:"size:40;index" json:"hash"` // SHA1 of cert in hex
	Reason     string         `gorm:"type:text" json:"reason"`
	Start      time.Time      `gorm:"not null" json:"start"`
	Duration   uint32         `gorm:"not null;default:0" json:"duration"` // 0 = permanent
	BannedBy   string         `gorm:"size:255" json:"banned_by"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name.
func (Ban) TableName() string {
	return "bans"
}
