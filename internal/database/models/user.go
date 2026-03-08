package models

import (
	"time"

	"gorm.io/gorm"
)

// Role represents user role.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// User represents a management API user account.
type User struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	Username       string         `gorm:"uniqueIndex;size:255;not null" json:"username"`
	PasswordHash   string         `gorm:"size:255;not null" json:"-"`
	JWTSecret      string         `gorm:"size:64;not null" json:"-"`
	Role           Role           `gorm:"size:32;not null;default:user" json:"role"`
	TokenVersion   int            `gorm:"not null;default:0" json:"-"`
	LastActivityAt *time.Time     `json:"last_activity_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name.
func (User) TableName() string {
	return "users"
}
