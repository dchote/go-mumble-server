package config

import (
	"errors"

	"github.com/dchote/go-mumble-server/internal/database/models"
	"gorm.io/gorm"
)

// MetaConfig holds process-level config loaded from DB.
type MetaConfig struct {
	SecurityMode  string
	Host          string
	MumblePort    int
	RESTPort      int
	Bonjour       bool
	RegisterName  string
	JWTIssuer     string
	JWTAudience   string
	JWTExpiryDays int
}

// ServerConfigData holds per-virtual-server config from DB.
type ServerConfigData struct {
	MaxUsers            int
	MaxBandwidth        int
	WelcomeText         string
	ServerPassword      string
	DefaultChannel      int
	CertRequired        bool
	ChannelNestingLimit int
	ChannelCountLimit   int
}

// LoadMetaConfig loads meta_config from DB (row ID 1).
func LoadMetaConfig(db *gorm.DB) (*MetaConfig, error) {
	var m models.MetaConfig
	if err := db.First(&m, 1).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &MetaConfig{
		SecurityMode:  m.SecurityMode,
		Host:          m.Host,
		MumblePort:    m.MumblePort,
		RESTPort:      m.RESTPort,
		Bonjour:       m.Bonjour,
		RegisterName:  m.RegisterName,
		JWTIssuer:     m.JWTIssuer,
		JWTAudience:   m.JWTAudience,
		JWTExpiryDays: m.JWTExpiryDays,
	}, nil
}

// UpdateMetaConfig updates meta_config in DB.
func UpdateMetaConfig(db *gorm.DB, m *MetaConfig) error {
	if m == nil {
		return nil
	}
	return db.Model(&models.MetaConfig{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"security_mode":   m.SecurityMode,
		"host":            m.Host,
		"mumble_port":     m.MumblePort,
		"rest_port":       m.RESTPort,
		"bonjour":         m.Bonjour,
		"register_name":   m.RegisterName,
		"jwt_issuer":      m.JWTIssuer,
		"jwt_audience":    m.JWTAudience,
		"jwt_expiry_days": m.JWTExpiryDays,
	}).Error
}

// LoadServerConfig loads per-server config from DB.
func LoadServerConfig(db *gorm.DB, serverID uint) (*ServerConfigData, error) {
	var sc models.ServerConfig
	if err := db.Where("server_id = ?", serverID).First(&sc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DefaultServerConfig(), nil
		}
		return nil, err
	}
	return &ServerConfigData{
		MaxUsers:            sc.MaxUsers,
		MaxBandwidth:        sc.MaxBandwidth,
		WelcomeText:         sc.WelcomeText,
		ServerPassword:      sc.ServerPassword,
		DefaultChannel:      sc.DefaultChannel,
		CertRequired:        sc.CertRequired,
		ChannelNestingLimit: sc.ChannelNestingLimit,
		ChannelCountLimit:   sc.ChannelCountLimit,
	}, nil
}

// DefaultServerConfig returns default server config values.
func DefaultServerConfig() *ServerConfigData {
	return &ServerConfigData{
		MaxUsers:            100,
		MaxBandwidth:        72000,
		ChannelNestingLimit: 10,
		ChannelCountLimit:   1000,
	}
}

// ConfigForServer builds a full Config from meta + server config for Mumble protocol use.
func ConfigForServer(meta *MetaConfig, server *ServerConfigData, bootstrap *Config) *Config {
	if meta == nil {
		meta = &MetaConfig{
			SecurityMode: "legacy",
			Host:         "0.0.0.0",
			MumblePort:   64738,
			RESTPort:     64730,
			JWTIssuer:    "go-mumble-server",
			JWTAudience:  "go-mumble-server-api",
			JWTExpiryDays: 30,
		}
	}
	if server == nil {
		server = DefaultServerConfig()
	}
	cfg := &Config{
		SecurityMode:   meta.SecurityMode,
		Host:           meta.Host,
		MumblePort:     meta.MumblePort,
		RESTPort:       meta.RESTPort,
		Bonjour:        meta.Bonjour,
		RegisterName:   meta.RegisterName,
		JWTIssuer:      meta.JWTIssuer,
		JWTAudience:    meta.JWTAudience,
		JWTExpiryDays:  meta.JWTExpiryDays,
		MaxUsers:       server.MaxUsers,
		MaxBandwidth:   server.MaxBandwidth,
		WelcomeText:    server.WelcomeText,
		ServerPassword: server.ServerPassword,
		DefaultChannel: server.DefaultChannel,
		CertRequired:   server.CertRequired,
		ChannelDepth:   server.ChannelNestingLimit,
		ChannelCount:   server.ChannelCountLimit,
	}
	if bootstrap != nil {
		cfg.DatabasePath = bootstrap.DatabasePath
		cfg.SSLCertPath = bootstrap.SSLCertPath
		cfg.SSLKeyPath = bootstrap.SSLKeyPath
		cfg.LogLevel = bootstrap.LogLevel
		cfg.FrontendEmbed = bootstrap.FrontendEmbed
	}
	if cfg.RegisterName == "" {
		cfg.RegisterName = "go-mumble-server"
	}
	return cfg
}

// EnsureMetaConfig seeds meta_config from cfg if not present.
func EnsureMetaConfig(db *gorm.DB, cfg *Config) error {
	var m models.MetaConfig
	err := db.First(&m, 1).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	m = models.MetaConfig{
		ID:            1,
		SecurityMode:  cfg.SecurityMode,
		Host:          cfg.Host,
		MumblePort:    cfg.MumblePort,
		RESTPort:      cfg.RESTPort,
		Bonjour:       cfg.Bonjour,
		RegisterName:  cfg.RegisterName,
		JWTIssuer:     cfg.JWTIssuer,
		JWTAudience:   cfg.JWTAudience,
		JWTExpiryDays: cfg.JWTExpiryDays,
	}
	if m.RegisterName == "" {
		m.RegisterName = "go-mumble-server"
	}
	return db.Create(&m).Error
}

// EnsureServerConfig seeds server_config for serverID if not present.
func EnsureServerConfig(db *gorm.DB, serverID uint, cfg *Config) error {
	var sc models.ServerConfig
	err := db.Where("server_id = ?", serverID).First(&sc).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	sc = models.ServerConfig{
		ServerID:            serverID,
		WelcomeText:         cfg.WelcomeText,
		MaxUsers:            cfg.MaxUsers,
		MaxBandwidth:        cfg.MaxBandwidth,
		ChannelNestingLimit: cfg.ChannelDepth,
		ChannelCountLimit:   cfg.ChannelCount,
		DefaultChannel:      cfg.DefaultChannel,
		CertRequired:        cfg.CertRequired,
		ServerPassword:      cfg.ServerPassword,
	}
	return db.Create(&sc).Error
}
