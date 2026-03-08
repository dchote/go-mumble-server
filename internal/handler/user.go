package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/dchote/go-mumble-server/internal/auth"
	"github.com/dchote/go-mumble-server/internal/httputil"
	"github.com/dchote/go-mumble-server/internal/service"
	"github.com/go-chi/chi/v5"
)

// UserHandler handles user management endpoints (admin only).
type UserHandler struct {
	user *service.UserService
}

// NewUserHandler creates a UserHandler.
func NewUserHandler(user *service.UserService) *UserHandler {
	return &UserHandler{user: user}
}

// List handles GET /users.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.user.ListUsers()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list users", "INTERNAL_ERROR", nil)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(users)
}

// Delete handles DELETE /users/{id}.
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid user id", "INVALID_REQUEST", nil)
		return
	}

	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	if err := h.user.DeleteUser(uint(id), claims.UserID); err != nil {
		if errors.Is(err, service.ErrCannotDeleteSelf) {
			httputil.WriteError(w, http.StatusBadRequest, "cannot delete self", "BAD_REQUEST", nil)
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to delete user", "INTERNAL_ERROR", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Update handles PATCH /users/{id}.
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid user id", "INVALID_REQUEST", nil)
		return
	}

	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST", nil)
		return
	}

	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	if err := h.user.UpdateUserRole(uint(id), body.Role, claims.UserID); err != nil {
		if errors.Is(err, service.ErrCannotChangeOwnRole) {
			httputil.WriteError(w, http.StatusBadRequest, "cannot change own role", "BAD_REQUEST", nil)
			return
		}
		if errors.Is(err, service.ErrInvalidRole) {
			httputil.WriteError(w, http.StatusBadRequest, "invalid role", "BAD_REQUEST", nil)
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update user", "INTERNAL_ERROR", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
