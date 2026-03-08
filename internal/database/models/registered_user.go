package models

import (
	"time"

	"gorm.io/gorm"
)

// RegisteredUser represents a Mumble-registered user (cert-based identity).
type RegisteredUser struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	ServerID       uint           `gorm:"index;not null" json:"server_id"`
	UserID         int32          `gorm:"not null" json:"user_id"` // Mumble user id
	Name           string         `gorm:"size:255;not null" json:"name"`
	CertHash       string         `gorm:"size:64;index" json:"-"` // SHA-256 of cert
	PasswordHash   string         `gorm:"size:255" json:"-"`      // Argon2id
	Email          string         `gorm:"size:255" json:"email"`
	LastChannelID  uint32         `gorm:"default:0" json:"last_channel_id"`
	LastActive     *time.Time     `json:"last_active"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name.
func (RegisteredUser) TableName() string {
	return "registered_users"
}
