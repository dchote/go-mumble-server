package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/dchote/go-mumble-server/internal/config"
	"github.com/dchote/go-mumble-server/internal/database"
	"github.com/dchote/go-mumble-server/internal/database/models"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// ServerHandler handles server and channel REST endpoints.
type ServerHandler struct {
	db             *gorm.DB
	cfg            *config.Config
	connectedUsers  interface{ ListConnected() interface{} }
}

// NewServerHandler creates a ServerHandler.
func NewServerHandler(db *gorm.DB, cfg *config.Config, connectedUsers interface{ ListConnected() interface{} }) *ServerHandler {
	return &ServerHandler{db: db, cfg: cfg, connectedUsers: connectedUsers}
}

// List returns all virtual servers. Seeds a default server if none exist.
func (h *ServerHandler) List(w http.ResponseWriter, r *http.Request) {
	if err := database.EnsureDefaultVirtualServer(h.db, "Default", h.cfg.Host, h.cfg.MumblePort, h.cfg.MaxUsers, h.cfg.WelcomeText); err != nil {
		http.Error(w, `{"error":"failed to ensure default server"}`, http.StatusInternalServerError)
		return
	}
	var servers []models.VirtualServer
	if err := h.db.Find(&servers).Error; err != nil {
		http.Error(w, `{"error":"failed to list servers"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(servers)
}

// GetChannels returns the channel tree for a server. Seeds root channel if none exist.
func (h *ServerHandler) GetChannels(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	var channels []models.Channel
	if err := h.db.Where("server_id = ?", id).Order("position, id").Find(&channels).Error; err != nil {
		http.Error(w, `{"error":"failed to list channels"}`, http.StatusInternalServerError)
		return
	}
	if len(channels) == 0 {
		root := models.Channel{
			ID:         0,
			ServerID:   uint(id),
			ParentID:   nil,
			Name:       "Root",
			Position:   0,
			InheritACL: true,
		}
		if err := h.db.Create(&root).Error; err == nil {
			channels = append(channels, root)
		}
	}
	tree := buildChannelTree(channels, nil)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

// ChannelNode is a channel with nested children for tree display.
type ChannelNode struct {
	ID          uint         `json:"id"`
	ServerID    uint         `json:"server_id"`
	ParentID    *uint        `json:"parent_id,omitempty"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Position    int32        `json:"position"`
	MaxUsers    uint32       `json:"max_users"`
	IsTemporary bool         `json:"is_temporary"`
	Children    []ChannelNode `json:"children,omitempty"`
}

func buildChannelTree(channels []models.Channel, parentID *uint) []ChannelNode {
	var out []ChannelNode
	for _, c := range channels {
		if (parentID == nil && c.ParentID == nil) || (parentID != nil && c.ParentID != nil && *c.ParentID == *parentID) {
			n := ChannelNode{
				ID:          c.ID,
				ServerID:    c.ServerID,
				ParentID:    c.ParentID,
				Name:        c.Name,
				Description: c.Description,
				Position:    c.Position,
				MaxUsers:    c.MaxUsers,
				IsTemporary: c.IsTemporary,
				Children:    buildChannelTree(channels, ptr(c.ID)),
			}
			out = append(out, n)
		}
	}
	return out
}

func ptr(u uint) *uint { return &u }

// GetUsers returns connected Mumble users for a server.
func (h *ServerHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	var users interface{} = []struct{}{}
	if h.connectedUsers != nil {
		users = h.connectedUsers.ListConnected()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// Status returns overall server statistics.
func (h *ServerHandler) Status(w http.ResponseWriter, r *http.Request) {
	var serverCount, channelCount int64
	h.db.Model(&models.VirtualServer{}).Count(&serverCount)
	h.db.Model(&models.Channel{}).Count(&channelCount)
	resp := map[string]interface{}{
		"status":        "ok",
		"servers":       serverCount,
		"channels":      channelCount,
		"mumble_port":   h.cfg.MumblePort,
		"rest_port":     h.cfg.RESTPort,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
