package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dchote/go-mumble-server/internal/auth"
	"github.com/dchote/go-mumble-server/internal/config"
	"github.com/dchote/go-mumble-server/internal/httputil"
	"github.com/dchote/go-mumble-server/internal/middleware"
	"github.com/dchote/go-mumble-server/internal/service"
	"gorm.io/gorm"
)

// AuthHandler handles auth endpoints.
type AuthHandler struct {
	user *service.UserService
	db   *gorm.DB
	cfg  *config.Config
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(user *service.UserService, db *gorm.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{user: user, db: db, cfg: cfg}
}

// RegisterRequest is the request body for register.
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginRequest is the request body for login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Register handles POST /auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST", nil)
		return
	}

	resp, err := h.user.Register(service.RegisterInput{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, service.ErrUsernameExists) {
			httputil.WriteError(w, http.StatusConflict, "username already exists", "USERNAME_EXISTS", nil)
			return
		}
		httputil.WriteError(w, http.StatusBadRequest, err.Error(), "BAD_REQUEST", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("auth register encode", "err", err)
	}
}

// Login handles POST /auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST", nil)
		return
	}

	resp, err := h.user.Login(service.LoginInput{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			httputil.WriteError(w, http.StatusUnauthorized, "invalid credentials", "INVALID_CREDENTIALS", nil)
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "login failed", "INTERNAL_ERROR", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("auth login encode", "err", err)
	}
}

// Status handles GET /auth/status.
// With valid Authorization: returns { user: { id, username, role } } or 401.
// Without auth: returns { hasUsers: bool }.
func (h *AuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			jwtCfg := auth.Config{
				Issuer:     h.cfg.JWTIssuer,
				Audience:   h.cfg.JWTAudience,
				ExpiryDays: h.cfg.JWTExpiryDays,
			}
			claims, err := middleware.ParseAndValidateToken(h.db, parts[1], jwtCfg)
			if err == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"user": map[string]interface{}{
						"id":       claims.UserID,
						"username": claims.Username,
						"role":     claims.Role,
					},
				})
				return
			}
		}
	}

	hasUsers, err := h.user.HasUsers()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to check status", "INTERNAL_ERROR", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{"hasUsers": hasUsers})
}
