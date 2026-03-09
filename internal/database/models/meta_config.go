package models

// MetaConfig represents process-level (global) configuration.
// Single row (ID=1); used for bind addresses, security mode, bonjour, JWT.
type MetaConfig struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	SecurityMode  string `gorm:"size:32;not null;default:legacy" json:"security_mode"`
	Host          string `gorm:"size:255;not null;default:0.0.0.0" json:"host"`
	MumblePort    int    `gorm:"not null;default:64738" json:"mumble_port"`
	RESTPort      int    `gorm:"not null;default:64730" json:"rest_port"`
	Bonjour       bool   `gorm:"not null;default:false" json:"bonjour"`
	RegisterName  string `gorm:"size:255" json:"register_name"`
	JWTIssuer     string `gorm:"size:255;not null;default:go-mumble-server" json:"-"`
	JWTAudience   string `gorm:"size:255;not null;default:go-mumble-server-api" json:"-"`
	JWTExpiryDays int    `gorm:"not null;default:30" json:"-"`
}

// TableName returns the table name.
func (MetaConfig) TableName() string {
	return "meta_config"
}
