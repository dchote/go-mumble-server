package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\"): %v", err)
	}
	if cfg.MumblePort != 64738 {
		t.Errorf("MumblePort = %d, want 64738", cfg.MumblePort)
	}
	if cfg.RESTPort != 64730 {
		t.Errorf("RESTPort = %d, want 64730", cfg.RESTPort)
	}
	if cfg.DatabasePath != "mumble-server.sqlite" {
		t.Errorf("DatabasePath = %q, want mumble-server.sqlite", cfg.DatabasePath)
	}
}

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mumble-server.toml")
	toml := `
[network]
port = 64739
rest_port = 9091
host = "127.0.0.1"

[database]
path = "/var/lib/mumble/db.sqlite"

[logging]
level = "debug"
`
	if err := os.WriteFile(path, []byte(toml), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MumblePort != 64739 {
		t.Errorf("MumblePort = %d, want 64739", cfg.MumblePort)
	}
	if cfg.RESTPort != 9091 {
		t.Errorf("RESTPort = %d, want 9091", cfg.RESTPort)
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want 127.0.0.1", cfg.Host)
	}
	if cfg.DatabasePath != "/var/lib/mumble/db.sqlite" {
		t.Errorf("DatabasePath = %q, want /var/lib/mumble/db.sqlite", cfg.DatabasePath)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.LogLevel)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mumble-server.toml")
	if err := os.WriteFile(path, []byte("[network]\nport = 64738\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	os.Setenv("MUMBLE_MUMBLE_PORT", "64740")
	defer os.Unsetenv("MUMBLE_MUMBLE_PORT")
	os.Setenv("MUMBLE_LOG_LEVEL", "warn")
	defer os.Unsetenv("MUMBLE_LOG_LEVEL")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MumblePort != 64740 {
		t.Errorf("MumblePort = %d, want 64740 (from env)", cfg.MumblePort)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want warn (from env)", cfg.LogLevel)
	}
}

func TestLoad_NonexistentFile(t *testing.T) {
	_, err := Load("/nonexistent/mumble-server.toml")
	if err == nil {
		t.Fatal("Load expected error for nonexistent file")
	}
}
