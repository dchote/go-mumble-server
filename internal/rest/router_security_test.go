package rest

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/dchote/go-mumble-server/internal/auth"
	"github.com/dchote/go-mumble-server/internal/config"
	"github.com/dchote/go-mumble-server/internal/database"
	"github.com/dchote/go-mumble-server/internal/database/models"
	"gorm.io/gorm"
)

func TestRouterRoleEnforcement(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "rest-security.sqlite")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	cfg := &config.Config{
		JWTIssuer:     "go-mumble-server",
		JWTAudience:   "go-mumble-server-api",
		JWTExpiryDays: 30,
	}

	adminToken := mustCreateAPIToken(t, db, cfg, "admin", models.RoleAdmin)
	userToken := mustCreateAPIToken(t, db, cfg, "readonly", models.RoleUser)
	handler := RouterWithMumble(db, cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	t.Run("non-admin can read server list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("non-admin cannot create server", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/servers", bytes.NewBufferString(`{"name":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("non-admin can read connected users (sanitized)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/servers/1/users", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		// Response may be empty array; non-admin must not receive sensitive fields (handled by handler when userLister is non-nil)
	})

	t.Run("admin can create server", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/servers", bytes.NewBufferString(`{"name":"admin-server"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", rr.Code)
		}
	})
}

func mustCreateAPIToken(t *testing.T, db *gorm.DB, cfg *config.Config, username string, role models.Role) string {
	t.Helper()

	secret, err := auth.GenerateSecret()
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	u := models.User{
		Username:     username,
		PasswordHash: "unused",
		JWTSecret:    secret,
		Role:         role,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := auth.Sign(auth.Config{
		Issuer:     cfg.JWTIssuer,
		Audience:   cfg.JWTAudience,
		ExpiryDays: cfg.JWTExpiryDays,
	}, u.ID, u.Username, string(u.Role), u.TokenVersion, u.JWTSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}
