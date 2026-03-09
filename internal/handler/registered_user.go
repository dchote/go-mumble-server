package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/dchote/go-mumble-server/internal/auth"
	"github.com/dchote/go-mumble-server/internal/database/models"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// RegisteredUserHandler handles Mumble registered-user REST endpoints.
type RegisteredUserHandler struct {
	db *gorm.DB
}

// NewRegisteredUserHandler creates a RegisteredUserHandler.
func NewRegisteredUserHandler(db *gorm.DB) *RegisteredUserHandler {
	return &RegisteredUserHandler{db: db}
}

// List returns registered users for a server.
func (h *RegisteredUserHandler) List(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	serverID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	var users []models.RegisteredUser
	if err := h.db.Where("server_id = ?", serverID).Find(&users).Error; err != nil {
		http.Error(w, `{"error":"failed to list users"}`, http.StatusInternalServerError)
		return
	}
	type resp struct {
		ID            uint   `json:"id"`
		UserID        int32  `json:"user_id"`
		Name          string `json:"name"`
		Email         string `json:"email"`
		LastChannelID uint32 `json:"last_channel_id"`
	}
	out := make([]resp, 0, len(users))
	for _, u := range users {
		out = append(out, resp{
			ID: u.ID, UserID: u.UserID, Name: u.Name, Email: u.Email, LastChannelID: u.LastChannelID,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// Create registers a new user.
func (h *RegisteredUserHandler) Create(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	serverID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		Name     string `json:"name"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}
	if body.Password == "" {
		http.Error(w, `{"error":"password is required"}`, http.StatusBadRequest)
		return
	}
	var maxID sql.NullInt32
	h.db.Model(&models.RegisteredUser{}).Where("server_id = ?", serverID).Select("MAX(user_id)").Scan(&maxID)
	nextID := int32(1)
	if maxID.Valid && maxID.Int32 >= 0 {
		nextID = maxID.Int32 + 1
	}
	passwordHash, err := auth.HashArgon2id(body.Password)
	if err != nil {
		http.Error(w, `{"error":"failed to hash password"}`, http.StatusInternalServerError)
		return
	}
	user := models.RegisteredUser{
		ServerID:     uint(serverID),
		UserID:       nextID,
		Name:         body.Name,
		PasswordHash: passwordHash,
		Email:        body.Email,
	}
	if err := h.db.Create(&user).Error; err != nil {
		http.Error(w, `{"error":"failed to create user"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": user.ID, "user_id": user.UserID, "name": user.Name, "email": user.Email,
	})
}

// Update updates a registered user.
func (h *RegisteredUserHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	serverID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	userIDStr := chi.URLParam(r, "userId")
	userID, err := strconv.ParseInt(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid user id"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		Name     *string `json:"name"`
		Password *string `json:"password"`
		Email    *string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	var user models.RegisteredUser
	if err := h.db.Where("server_id = ? AND user_id = ?", serverID, userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to get user"}`, http.StatusInternalServerError)
		return
	}
	updates := map[string]interface{}{}
	if body.Name != nil {
		updates["name"] = *body.Name
	}
	if body.Password != nil && *body.Password != "" {
		hash, err := auth.HashArgon2id(*body.Password)
		if err != nil {
			http.Error(w, `{"error":"failed to hash password"}`, http.StatusInternalServerError)
			return
		}
		updates["password_hash"] = hash
	}
	if body.Email != nil {
		updates["email"] = *body.Email
	}
	if len(updates) > 0 {
		if err := h.db.Model(&user).Updates(updates).Error; err != nil {
			http.Error(w, `{"error":"failed to update user"}`, http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// Delete unregisters a user.
func (h *RegisteredUserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	serverID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	userIDStr := chi.URLParam(r, "userId")
	userID, err := strconv.ParseInt(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid user id"}`, http.StatusBadRequest)
		return
	}
	if err := h.db.Where("server_id = ? AND user_id = ?", serverID, userID).Delete(&models.RegisteredUser{}).Error; err != nil {
		http.Error(w, `{"error":"failed to delete user"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
