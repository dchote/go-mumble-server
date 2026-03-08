package handler

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/dchote/go-mumble-server/internal/database/models"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// BanHandler handles ban REST endpoints.
type BanHandler struct {
	db *gorm.DB
}

// NewBanHandler creates a BanHandler.
func NewBanHandler(db *gorm.DB) *BanHandler {
	return &BanHandler{db: db}
}

// List returns all bans for a server.
func (h *BanHandler) List(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	var bans []models.Ban
	if err := h.db.Where("server_id = ?", id).Find(&bans).Error; err != nil {
		http.Error(w, `{"error":"failed to list bans"}`, http.StatusInternalServerError)
		return
	}
	type banResp struct {
		ID       uint   `json:"id"`
		Address  string `json:"address"`
		Mask     uint32 `json:"mask"`
		Name     string `json:"name"`
		Hash     string `json:"hash"`
		Reason   string `json:"reason"`
		Start    string `json:"start"`
		Duration uint32 `json:"duration"`
		BannedBy string `json:"banned_by"`
	}
	out := make([]banResp, 0, len(bans))
	for _, b := range bans {
		addr := ""
		if len(b.Address) == 4 {
			addr = net.IP(b.Address).To4().String()
		} else if len(b.Address) == 16 {
			addr = net.IP(b.Address).String()
		}
		out = append(out, banResp{
			ID:       b.ID,
			Address:  addr,
			Mask:     b.Mask,
			Name:     b.Name,
			Hash:     b.Hash,
			Reason:   b.Reason,
			Start:    b.Start.Format(time.RFC3339),
			Duration: b.Duration,
			BannedBy: b.BannedBy,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// Create adds a ban.
func (h *BanHandler) Create(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	serverID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		Address  string `json:"address"`
		Hash     string `json:"hash"`
		Name     string `json:"name"`
		Reason   string `json:"reason"`
		Duration uint32 `json:"duration"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if body.Address == "" && body.Hash == "" {
		http.Error(w, `{"error":"address or hash required"}`, http.StatusBadRequest)
		return
	}
	var addr []byte
	mask := uint32(32)
	if body.Address != "" {
		ip := net.ParseIP(body.Address)
		if ip == nil {
			http.Error(w, `{"error":"invalid address"}`, http.StatusBadRequest)
			return
		}
		addr = ip.To4()
		if addr == nil {
			addr = ip.To16()
			mask = 128
		}
	}
	ban := models.Ban{
		ServerID: uint(serverID),
		Address:  addr,
		Mask:     mask,
		Name:     body.Name,
		Hash:     body.Hash,
		Reason:   body.Reason,
		Start:    time.Now(),
		Duration: body.Duration,
		BannedBy: "api",
	}
	if err := h.db.Create(&ban).Error; err != nil {
		http.Error(w, `{"error":"failed to create ban"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": ban.ID, "address": body.Address, "hash": ban.Hash, "name": ban.Name,
		"reason": ban.Reason, "duration": ban.Duration, "banned_by": ban.BannedBy,
	})
}

// Delete removes a ban.
func (h *BanHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	serverID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	banIDStr := chi.URLParam(r, "banId")
	banID, err := strconv.ParseUint(banIDStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid ban id"}`, http.StatusBadRequest)
		return
	}
	var ban models.Ban
	if err := h.db.Where("id = ? AND server_id = ?", banID, serverID).First(&ban).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, `{"error":"ban not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to delete ban"}`, http.StatusInternalServerError)
		return
	}
	if err := h.db.Delete(&ban).Error; err != nil {
		http.Error(w, `{"error":"failed to delete ban"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
