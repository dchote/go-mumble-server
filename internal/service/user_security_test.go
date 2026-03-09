package service

import (
	"path/filepath"
	"testing"

	"github.com/dchote/go-mumble-server/internal/auth"
	"github.com/dchote/go-mumble-server/internal/config"
	"github.com/dchote/go-mumble-server/internal/database"
	"github.com/dchote/go-mumble-server/internal/database/models"
)

func TestUpdateUserRoleInvalidatesExistingTokens(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "security.sqlite")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	cfg := &config.Config{
		JWTIssuer:     "go-mumble-server",
		JWTAudience:   "go-mumble-server-api",
		JWTExpiryDays: 30,
	}
	svc := NewUserService(db, cfg)

	adminSecret, err := auth.GenerateSecret()
	if err != nil {
		t.Fatalf("generate admin secret: %v", err)
	}
	userSecret, err := auth.GenerateSecret()
	if err != nil {
		t.Fatalf("generate user secret: %v", err)
	}

	admin := models.User{Username: "admin", PasswordHash: "x", JWTSecret: adminSecret, Role: models.RoleAdmin}
	target := models.User{Username: "user", PasswordHash: "x", JWTSecret: userSecret, Role: models.RoleUser}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := db.Create(&target).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := auth.Sign(auth.Config{
		Issuer:     cfg.JWTIssuer,
		Audience:   cfg.JWTAudience,
		ExpiryDays: cfg.JWTExpiryDays,
	}, target.ID, target.Username, string(target.Role), target.TokenVersion, target.JWTSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	if err := svc.UpdateUserRole(target.ID, string(models.RoleAdmin), admin.ID); err != nil {
		t.Fatalf("update role: %v", err)
	}

	var updated models.User
	if err := db.First(&updated, target.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if updated.TokenVersion <= target.TokenVersion {
		t.Fatalf("expected token version to increment, got %d", updated.TokenVersion)
	}
	if updated.JWTSecret == target.JWTSecret {
		t.Fatalf("expected JWT secret to rotate on role update")
	}
	if _, err := auth.Validate(token, updated.JWTSecret, auth.Config{
		Issuer:     cfg.JWTIssuer,
		Audience:   cfg.JWTAudience,
		ExpiryDays: cfg.JWTExpiryDays,
	}); err == nil {
		t.Fatalf("expected old token to be invalid after secret rotation")
	}
}
