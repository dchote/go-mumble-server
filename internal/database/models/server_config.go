package models

// ServerConfig represents per-virtual-server configuration.
type ServerConfig struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	ServerID     uint   `gorm:"uniqueIndex;not null" json:"server_id"`
	WelcomeText  string `gorm:"type:text" json:"welcome_text"`
	MaxUsers     int    `gorm:"not null;default:100" json:"max_users"`
	MaxBandwidth int    `gorm:"not null;default:72000" json:"max_bandwidth"`
	AllowHTML    bool   `gorm:"not null;default:true" json:"allow_html"`
	RegName      string `gorm:"size:255" json:"reg_name"`
	RegPassword  string `gorm:"size:255" json:"-"`
	RegURL       string `gorm:"type:text" json:"reg_url"`
}

// TableName returns the table name.
func (ServerConfig) TableName() string {
	return "server_configs"
}
