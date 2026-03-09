package service

import (
	"errors"
	"time"

	"github.com/dchote/go-mumble-server/internal/auth"
	"github.com/dchote/go-mumble-server/internal/config"
	"github.com/dchote/go-mumble-server/internal/database/models"
	"github.com/dchote/go-mumble-server/internal/sanitize"
	"gorm.io/gorm"
)

var (
	ErrUsernameExists      = errors.New("username already exists")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrCannotChangeOwnRole = errors.New("cannot change own role")
	ErrCannotDeleteSelf    = errors.New("cannot delete self")
	ErrInvalidRole         = errors.New("invalid role")
)

// UserService handles user business logic.
type UserService struct {
	db   *gorm.DB
	cfg  *config.Config
	auth *auth.Config
}

// NewUserService creates a UserService.
func NewUserService(db *gorm.DB, cfg *config.Config) *UserService {
	return &UserService{
		db:  db,
		cfg: cfg,
		auth: &auth.Config{
			Issuer:     cfg.JWTIssuer,
			Audience:   cfg.JWTAudience,
			ExpiryDays: cfg.JWTExpiryDays,
		},
	}
}

// RegisterInput is the input for Register.
type RegisterInput struct {
	Username string
	Password string
}

// LoginResponse is the response for login/register.
type LoginResponse struct {
	Token string       `json:"token"`
	User  *UserPayload `json:"user"`
}

// UserPayload is the user data returned in API responses.
type UserPayload struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// Register creates a new user. First user becomes admin.
func (s *UserService) Register(in RegisterInput) (*LoginResponse, error) {
	username := sanitize.Username(in.Username)
	if username == "" {
		return nil, errors.New("username required")
	}
	if len(in.Password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	var count int64
	if err := s.db.Model(&models.User{}).Count(&count).Error; err != nil {
		return nil, err
	}

	role := models.RoleUser
	if count == 0 {
		role = models.RoleAdmin
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}

	secret, err := auth.GenerateSecret()
	if err != nil {
		return nil, err
	}

	var existing models.User
	if err := s.db.Where("username = ?", username).First(&existing).Error; err == nil {
		return nil, ErrUsernameExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	user := models.User{
		Username:     username,
		PasswordHash: hash,
		JWTSecret:    secret,
		Role:         role,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}

	now := time.Now()
	_ = s.db.Model(&user).Update("last_activity_at", now)

	token, err := auth.Sign(*s.auth, user.ID, user.Username, string(user.Role), user.TokenVersion, user.JWTSecret)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		Token: token,
		User: &UserPayload{
			ID:       user.ID,
			Username: user.Username,
			Role:     string(user.Role),
		},
	}, nil
}

// LoginInput is the input for Login.
type LoginInput struct {
	Username string
	Password string
}

// Login authenticates a user and returns a token.
func (s *UserService) Login(in LoginInput) (*LoginResponse, error) {
	username := sanitize.Username(in.Username)
	if username == "" {
		return nil, ErrInvalidCredentials
	}

	var user models.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !auth.ComparePassword(user.PasswordHash, in.Password) {
		return nil, ErrInvalidCredentials
	}

	token, err := auth.Sign(*s.auth, user.ID, user.Username, string(user.Role), user.TokenVersion, user.JWTSecret)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	_ = s.db.Model(&models.User{}).Where("id = ?", user.ID).Update("last_activity_at", now)

	return &LoginResponse{
		Token: token,
		User: &UserPayload{
			ID:       user.ID,
			Username: user.Username,
			Role:     string(user.Role),
		},
	}, nil
}

// UpdateLastActivity sets LastActivityAt for the user.
func (s *UserService) UpdateLastActivity(userID uint) error {
	now := time.Now()
	return s.db.Model(&models.User{}).Where("id = ?", userID).Update("last_activity_at", now).Error
}

// HasUsers returns true if any users exist.
func (s *UserService) HasUsers() (bool, error) {
	var count int64
	if err := s.db.Model(&models.User{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// AuthUser holds minimal user data for token validation.
type AuthUser struct {
	JWTSecret    string
	TokenVersion int
}

// GetUserForAuth returns user auth fields for token validation.
func GetUserForAuth(db *gorm.DB, userID uint) (*AuthUser, error) {
	var u struct {
		JWTSecret    string
		TokenVersion int
	}
	if err := db.Model(&models.User{}).Where("id = ?", userID).Select("jwt_secret", "token_version").First(&u).Error; err != nil {
		return nil, err
	}
	return &AuthUser{JWTSecret: u.JWTSecret, TokenVersion: u.TokenVersion}, nil
}

// ListUsers returns all users (admin only).
func (s *UserService) ListUsers() ([]UserPayload, error) {
	var users []models.User
	if err := s.db.Order("username").Find(&users).Error; err != nil {
		return nil, err
	}
	out := make([]UserPayload, len(users))
	for i, u := range users {
		out[i] = UserPayload{ID: u.ID, Username: u.Username, Role: string(u.Role)}
	}
	return out, nil
}

// UpdateUserRole updates a user's role (admin only). Cannot change own role.
func (s *UserService) UpdateUserRole(id uint, newRole string, currentUserID uint) error {
	if id == currentUserID {
		return ErrCannotChangeOwnRole
	}
	r := models.Role(newRole)
	if r != models.RoleAdmin && r != models.RoleUser {
		return ErrInvalidRole
	}
	newSecret, err := auth.GenerateSecret()
	if err != nil {
		return err
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		return tx.Model(&models.User{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"role":          r,
				"jwt_secret":    newSecret,
				"token_version": gorm.Expr("token_version + 1"),
			}).Error
	})
}

// DeleteUser deletes a user (admin only). Cannot delete self.
func (s *UserService) DeleteUser(id uint, currentUserID uint) error {
	if id == currentUserID {
		return ErrCannotDeleteSelf
	}
	return s.db.Delete(&models.User{}, id).Error
}
