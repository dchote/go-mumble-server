package database

import (
	"errors"
	"log/slog"

	"github.com/dchote/go-mumble-server/internal/database/models"
	"gorm.io/gorm"
)

// EnsureDefaultVirtualServer ensures virtual server 1 exists. Creates it with the
// given params if not found. Returns an error only if the database call fails.
func EnsureDefaultVirtualServer(db *gorm.DB, name, host string, port, maxUsers int, welcomeText string) error {
	var vs models.VirtualServer
	err := db.First(&vs, 1).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Warn("failed to check for default virtual server", "err", err)
		return err
	}
	vs = models.VirtualServer{
		ID:          1,
		Name:        name,
		Host:        host,
		Port:        port,
		MaxUsers:    maxUsers,
		WelcomeText: welcomeText,
	}
	if err := db.Create(&vs).Error; err != nil {
		slog.Warn("failed to create default virtual server", "err", err)
		return err
	}
	return nil
}
