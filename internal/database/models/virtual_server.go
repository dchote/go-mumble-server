package models

import (
	"time"

	"gorm.io/gorm"
)

// VirtualServer represents a Mumble virtual server instance.
type VirtualServer struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:255" json:"name"`
	Host        string         `gorm:"size:255" json:"host"`
	Port        int            `gorm:"not null" json:"port"`
	MaxUsers    int            `gorm:"not null;default:100" json:"max_users"`
	WelcomeText string         `gorm:"type:text" json:"welcome_text"`
	Password    string         `gorm:"size:255" json:"-"` // Server password (hashed or plain per config)
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name.
func (VirtualServer) TableName() string {
	return "virtual_servers"
}
