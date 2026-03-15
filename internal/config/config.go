package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds the server configuration.
type Config struct {
	Host           string
	MumblePort     int
	RESTPort       int
	FrontendEmbed  bool
	DatabasePath   string
	SSLCertPath    string
	SSLKeyPath     string
	MaxUsers       int
	MaxBandwidth   int
	LogLevel       string
	JWTIssuer      string
	JWTAudience    string
	JWTExpiryDays  int
	ChannelDepth   int
	ChannelCount   int
	WelcomeText    string
	ServerPassword string
	DefaultChannel int
	CertRequired   bool
	Bonjour        bool
	RegisterName   string
	VoiceDebug     bool
}

// fileConfig mirrors the TOML structure for parsing.
type fileConfig struct {
	Network struct {
		Port     int    `toml:"port"`
		RestPort int    `toml:"rest_port"`
		Host     string `toml:"host"`
	} `toml:"network"`
	TLS struct {
		Cert string `toml:"cert"`
		Key  string `toml:"key"`
	} `toml:"tls"`
	Server struct {
		MaxUsers       int    `toml:"max_users"`
		MaxBandwidth   int    `toml:"max_bandwidth"`
		WelcomeText    string `toml:"welcome_text"`
		ServerPassword string `toml:"server_password"`
	} `toml:"server"`
	Database struct {
		Path string `toml:"path"`
	} `toml:"database"`
	Logging struct {
		Level string `toml:"level"`
	} `toml:"logging"`
	Channels struct {
		NestingLimit int `toml:"nesting_limit"`
		CountLimit   int `toml:"count_limit"`
	} `toml:"channels"`
	Users struct {
		DefaultChannel int  `toml:"default_channel"`
		CertRequired   bool `toml:"cert_required"`
	} `toml:"users"`
	Bonjour struct {
		Enabled bool   `toml:"enabled"`
		Name    string `toml:"register_name"`
	} `toml:"bonjour"`
}

// defaults returns the default configuration.
func defaults() *Config {
	return &Config{
		Host:          "0.0.0.0",
		MumblePort:    64738,
		RESTPort:      64730,
		FrontendEmbed: true,
		DatabasePath:  "mumble-server.sqlite",
		LogLevel:      "info",
		JWTIssuer:     "go-mumble-server",
		JWTAudience:   "go-mumble-server-api",
		JWTExpiryDays: 30,
		MaxUsers:      100,
		MaxBandwidth:  72000,
		ChannelDepth:  10,
		ChannelCount:  1000,
	}
}

// Load reads configuration with precedence: file (if path set) < env (MUMBLE_*) < defaults.
// Flags (e.g. -frontend-embed) are applied by the caller after Load.
func Load(path string) (*Config, error) {
	cfg := defaults()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config %s: %w", path, err)
		}
		var fc fileConfig
		if err := toml.Unmarshal(data, &fc); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
		applyFileConfig(cfg, &fc)
	}

	applyEnv(cfg)
	return cfg, nil
}

func applyFileConfig(cfg *Config, fc *fileConfig) {
	if fc.Network.Host != "" {
		cfg.Host = fc.Network.Host
	}
	if fc.Network.Port != 0 {
		cfg.MumblePort = fc.Network.Port
	}
	if fc.Network.RestPort != 0 {
		cfg.RESTPort = fc.Network.RestPort
	}
	if fc.TLS.Cert != "" {
		cfg.SSLCertPath = fc.TLS.Cert
	}
	if fc.TLS.Key != "" {
		cfg.SSLKeyPath = fc.TLS.Key
	}
	if fc.Server.MaxUsers != 0 {
		cfg.MaxUsers = fc.Server.MaxUsers
	}
	if fc.Server.MaxBandwidth != 0 {
		cfg.MaxBandwidth = fc.Server.MaxBandwidth
	}
	cfg.WelcomeText = fc.Server.WelcomeText
	cfg.ServerPassword = fc.Server.ServerPassword
	if fc.Database.Path != "" {
		cfg.DatabasePath = fc.Database.Path
	}
	if fc.Logging.Level != "" {
		cfg.LogLevel = fc.Logging.Level
	}
	if fc.Channels.NestingLimit != 0 {
		cfg.ChannelDepth = fc.Channels.NestingLimit
	}
	if fc.Channels.CountLimit != 0 {
		cfg.ChannelCount = fc.Channels.CountLimit
	}
	cfg.DefaultChannel = fc.Users.DefaultChannel
	cfg.CertRequired = fc.Users.CertRequired
	if fc.Bonjour.Enabled {
		cfg.Bonjour = true
	}
	if fc.Bonjour.Name != "" {
		cfg.RegisterName = fc.Bonjour.Name
	}
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("MUMBLE_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("MUMBLE_MUMBLE_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MumblePort = n
		}
	}
	if v := os.Getenv("MUMBLE_REST_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RESTPort = n
		}
	}
	if v := os.Getenv("MUMBLE_DATABASE_PATH"); v != "" {
		cfg.DatabasePath = v
	}
	if v := os.Getenv("MUMBLE_SSL_CERT_PATH"); v != "" {
		cfg.SSLCertPath = v
	}
	if v := os.Getenv("MUMBLE_SSL_KEY_PATH"); v != "" {
		cfg.SSLKeyPath = v
	}
	if v := os.Getenv("MUMBLE_MAX_USERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxUsers = n
		}
	}
	if v := os.Getenv("MUMBLE_MAX_BANDWIDTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxBandwidth = n
		}
	}
	if v := os.Getenv("MUMBLE_LOG_LEVEL"); v != "" {
		cfg.LogLevel = strings.ToLower(v)
	}
	if v := os.Getenv("MUMBLE_JWT_ISSUER"); v != "" {
		cfg.JWTIssuer = v
	}
	if v := os.Getenv("MUMBLE_JWT_AUDIENCE"); v != "" {
		cfg.JWTAudience = v
	}
	if v := os.Getenv("MUMBLE_JWT_EXPIRY_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.JWTExpiryDays = n
		}
	}
	if v := os.Getenv("MUMBLE_BONJOUR"); v != "" {
		cfg.Bonjour = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("MUMBLE_REGISTER_NAME"); v != "" {
		cfg.RegisterName = v
	}
	if v := os.Getenv("MUMBLE_WELCOME_TEXT"); v != "" {
		cfg.WelcomeText = v
	}
	if v := os.Getenv("MUMBLE_SERVER_PASSWORD"); v != "" {
		cfg.ServerPassword = v
	}
	if v := os.Getenv("MUMBLE_CHANNEL_NESTING_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ChannelDepth = n
		}
	}
	if v := os.Getenv("MUMBLE_CHANNEL_COUNT_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ChannelCount = n
		}
	}
	if v := os.Getenv("MUMBLE_DEFAULT_CHANNEL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.DefaultChannel = n
		}
	}
	if v := os.Getenv("MUMBLE_CERT_REQUIRED"); v != "" {
		cfg.CertRequired = strings.ToLower(v) == "true" || v == "1"
	}
}
