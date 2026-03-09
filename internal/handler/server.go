package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/dchote/go-mumble-server/internal/acl"
	"github.com/dchote/go-mumble-server/internal/cert"
	"github.com/dchote/go-mumble-server/internal/channel"
	"github.com/dchote/go-mumble-server/internal/config"
	"github.com/dchote/go-mumble-server/internal/database"
	"github.com/dchote/go-mumble-server/internal/database/models"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// OnChannelMutated is called when channels are created, updated, or deleted via REST.
// ch is the channel for create/update; for delete, ch is nil and channelID is the removed ID.
type OnChannelMutated func(serverID uint, ch interface{}, channelID uint32, removed bool)

// ConnectedUserLister lists connected Mumble users; used by GetUsers.
type ConnectedUserLister interface {
	ListConnected(serverID uint, db *gorm.DB) interface{}
}

// ConnectedUserActioner performs kick, mute, ban on connected users (REST-initiated).
type ConnectedUserActioner interface {
	Kick(serverID uint, sessionID uint32, reason string) (ok bool)
	Mute(serverID uint, sessionID uint32, mute bool) (ok bool)
	BanAndKick(serverID uint, sessionID uint32, reason string) (ok bool)
}

// ServerHandler handles server and channel REST endpoints.
type ServerHandler struct {
	db                  *gorm.DB
	cfg                 *config.Config
	connectedUsers      ConnectedUserLister
	userActioner        ConnectedUserActioner
	getChanMgr          func(serverID uint) *channel.Manager
	onChannelMutated    OnChannelMutated
	metaHost            string
	metaMumblePort      int
}

// NewServerHandler creates a ServerHandler.
func NewServerHandler(db *gorm.DB, cfg *config.Config, connectedUsers ConnectedUserLister, userActioner ConnectedUserActioner, getChanMgr func(serverID uint) *channel.Manager, onChannelMutated OnChannelMutated) *ServerHandler {
	meta, _ := config.LoadMetaConfig(db)
	host, port := "0.0.0.0", 64738
	if meta != nil {
		host, port = meta.Host, meta.MumblePort
	}
	return &ServerHandler{db: db, cfg: cfg, connectedUsers: connectedUsers, userActioner: userActioner, getChanMgr: getChanMgr, onChannelMutated: onChannelMutated, metaHost: host, metaMumblePort: port}
}

// List returns all virtual servers. Seeds a default server if none exist.
func (h *ServerHandler) List(w http.ResponseWriter, r *http.Request) {
	if err := database.EnsureDefaultVirtualServer(h.db, "Default", h.metaHost, h.metaMumblePort, 100, ""); err != nil {
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

// Get returns a single virtual server by ID.
func (h *ServerHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	var vs models.VirtualServer
	if err := h.db.First(&vs, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, `{"error":"server not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to get server"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vs)
}

// Create creates a new virtual server.
func (h *ServerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Host        string `json:"host"`
		Port        int    `json:"port"`
		MaxUsers    int    `json:"max_users"`
		WelcomeText string `json:"welcome_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		body.Name = "Server"
	}
	if body.Host == "" {
		body.Host = h.metaHost
	}
	if body.Port == 0 {
		body.Port = h.metaMumblePort
	}
	if body.MaxUsers <= 0 {
		body.MaxUsers = 100
	}
	vs := models.VirtualServer{
		Name:        body.Name,
		Host:        body.Host,
		Port:        body.Port,
		MaxUsers:    body.MaxUsers,
		WelcomeText: body.WelcomeText,
	}
	if err := h.db.Create(&vs).Error; err != nil {
		http.Error(w, `{"error":"failed to create server"}`, http.StatusInternalServerError)
		return
	}
	_, _, _ = cert.GetOrCreateCertForVirtualServer(h.db, vs.ID)
	_ = config.EnsureServerConfig(h.db, vs.ID, h.cfg)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(vs)
}

// Update updates a virtual server.
func (h *ServerHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		Name        *string `json:"name"`
		Host        *string `json:"host"`
		Port        *int    `json:"port"`
		MaxUsers    *int    `json:"max_users"`
		WelcomeText *string `json:"welcome_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	var vs models.VirtualServer
	if err := h.db.First(&vs, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, `{"error":"server not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to get server"}`, http.StatusInternalServerError)
		return
	}
	updates := map[string]interface{}{}
	if body.Name != nil {
		vs.Name = *body.Name
		updates["name"] = *body.Name
	}
	if body.Host != nil {
		vs.Host = *body.Host
		updates["host"] = *body.Host
	}
	if body.Port != nil {
		vs.Port = *body.Port
		updates["port"] = *body.Port
	}
	if body.MaxUsers != nil {
		vs.MaxUsers = *body.MaxUsers
		updates["max_users"] = *body.MaxUsers
	}
	if body.WelcomeText != nil {
		vs.WelcomeText = *body.WelcomeText
		updates["welcome_text"] = *body.WelcomeText
	}
	if len(updates) > 0 {
		if err := h.db.Model(&vs).Updates(updates).Error; err != nil {
			http.Error(w, `{"error":"failed to update server"}`, http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vs)
}

// GetConfig returns per-server config.
func (h *ServerHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	sc, err := config.LoadServerConfig(h.db, uint(id))
	if err != nil {
		http.Error(w, `{"error":"failed to load config"}`, http.StatusInternalServerError)
		return
	}
	if sc == nil {
		sc = config.DefaultServerConfig()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"max_users":            sc.MaxUsers,
		"max_bandwidth":        sc.MaxBandwidth,
		"welcome_text":         sc.WelcomeText,
		"default_channel":      sc.DefaultChannel,
		"cert_required":        sc.CertRequired,
		"channel_nesting_limit": sc.ChannelNestingLimit,
		"channel_count_limit":  sc.ChannelCountLimit,
	})
}

// UpdateConfig updates per-server config.
func (h *ServerHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		MaxUsers            *int    `json:"max_users"`
		MaxBandwidth        *int    `json:"max_bandwidth"`
		WelcomeText         *string `json:"welcome_text"`
		DefaultChannel      *int    `json:"default_channel"`
		CertRequired        *bool   `json:"cert_required"`
		ChannelNestingLimit *int    `json:"channel_nesting_limit"`
		ChannelCountLimit   *int    `json:"channel_count_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	var sc models.ServerConfig
	if err := h.db.Where("server_id = ?", id).First(&sc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sc = models.ServerConfig{ServerID: uint(id)}
			if err := h.db.Create(&sc).Error; err != nil {
				http.Error(w, `{"error":"failed to create config"}`, http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, `{"error":"failed to load config"}`, http.StatusInternalServerError)
			return
		}
	}
	updates := map[string]interface{}{}
	if body.MaxUsers != nil {
		updates["max_users"] = *body.MaxUsers
	}
	if body.MaxBandwidth != nil {
		updates["max_bandwidth"] = *body.MaxBandwidth
	}
	if body.WelcomeText != nil {
		updates["welcome_text"] = *body.WelcomeText
	}
	if body.DefaultChannel != nil {
		updates["default_channel"] = *body.DefaultChannel
	}
	if body.CertRequired != nil {
		updates["cert_required"] = *body.CertRequired
	}
	if body.ChannelNestingLimit != nil {
		updates["channel_nesting_limit"] = *body.ChannelNestingLimit
	}
	if body.ChannelCountLimit != nil {
		updates["channel_count_limit"] = *body.ChannelCountLimit
	}
	if len(updates) > 0 {
		if err := h.db.Model(&sc).Updates(updates).Error; err != nil {
			http.Error(w, `{"error":"failed to update config"}`, http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetMetaConfig returns global (meta) config.
func (h *ServerHandler) GetMetaConfig(w http.ResponseWriter, r *http.Request) {
	meta, err := config.LoadMetaConfig(h.db)
	if err != nil {
		http.Error(w, `{"error":"failed to load config"}`, http.StatusInternalServerError)
		return
	}
	if meta == nil {
		meta = &config.MetaConfig{
			SecurityMode:  "legacy",
			Host:          "0.0.0.0",
			MumblePort:    64738,
			RESTPort:      64730,
			JWTIssuer:     "go-mumble-server",
			JWTAudience:   "go-mumble-server-api",
			JWTExpiryDays: 30,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"security_mode": meta.SecurityMode, "host": meta.Host,
		"mumble_port": meta.MumblePort, "rest_port": meta.RESTPort,
		"bonjour": meta.Bonjour, "register_name": meta.RegisterName,
	})
}

// UpdateMetaConfig updates global (meta) config.
func (h *ServerHandler) UpdateMetaConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SecurityMode *string `json:"security_mode"`
		Host         *string `json:"host"`
		MumblePort   *int    `json:"mumble_port"`
		RESTPort     *int    `json:"rest_port"`
		Bonjour      *bool   `json:"bonjour"`
		RegisterName *string `json:"register_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	meta, err := config.LoadMetaConfig(h.db)
	if err != nil || meta == nil {
		meta = &config.MetaConfig{
			SecurityMode: "legacy", Host: "0.0.0.0",
			MumblePort: 64738, RESTPort: 64730,
			JWTIssuer: "go-mumble-server", JWTAudience: "go-mumble-server-api", JWTExpiryDays: 30,
		}
	}
	if body.SecurityMode != nil {
		meta.SecurityMode = *body.SecurityMode
	}
	if body.Host != nil {
		meta.Host = *body.Host
	}
	if body.MumblePort != nil {
		meta.MumblePort = *body.MumblePort
	}
	if body.RESTPort != nil {
		meta.RESTPort = *body.RESTPort
	}
	if body.Bonjour != nil {
		meta.Bonjour = *body.Bonjour
	}
	if body.RegisterName != nil {
		meta.RegisterName = *body.RegisterName
	}
	if err := config.UpdateMetaConfig(h.db, meta); err != nil {
		http.Error(w, `{"error":"failed to update config"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Delete removes a virtual server.
func (h *ServerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	var vs models.VirtualServer
	if err := h.db.First(&vs, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, `{"error":"server not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to get server"}`, http.StatusInternalServerError)
		return
	}
	if err := h.db.Delete(&vs).Error; err != nil {
		http.Error(w, `{"error":"failed to delete server"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
		// Select ID to force GORM to include it; otherwise ID 0 is omitted and DB auto-increments
		if err := h.db.Select("ID", "ServerID", "ParentID", "Name", "Position", "InheritACL").Create(&root).Error; err == nil {
			channels = append(channels, root)
		}
	}
	tree := buildChannelTree(channels, nil)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

func (h *ServerHandler) chanManager(serverID uint) *channel.Manager {
	if h.getChanMgr != nil {
		return h.getChanMgr(serverID)
	}
	mgr := channel.NewManager(h.db, serverID)
	_ = acl.EnsureDefaultRootACLs(h.db, serverID)
	return mgr
}

// CreateChannel creates a channel under a parent.
func (h *ServerHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		ParentID    uint32 `json:"parent_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Position    int32  `json:"position"`
		Temporary   bool   `json:"is_temporary"`
		MaxUsers    uint32 `json:"max_users"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}
	mgr := h.chanManager(uint(id))
	ch := mgr.Create(body.ParentID, body.Name, body.Description, body.Position, body.Temporary, body.MaxUsers)
	if ch == nil {
		http.Error(w, `{"error":"failed to create channel"}`, http.StatusInternalServerError)
		return
	}
	if h.onChannelMutated != nil {
		h.onChannelMutated(uint(id), ch, ch.ID, false)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": ch.ID, "parent_id": ch.ParentID, "name": ch.Name, "description": ch.Description,
		"position": ch.Position, "max_users": ch.MaxUsers, "is_temporary": ch.IsTemporary,
	})
}

// UpdateChannel updates a channel.
func (h *ServerHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	serverID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	chIDStr := chi.URLParam(r, "channelId")
	chID, err := strconv.ParseUint(chIDStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid channel id"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		Name        *string  `json:"name"`
		Description *string  `json:"description"`
		Position    *int32   `json:"position"`
		MaxUsers    *uint32  `json:"max_users"`
		Temporary   *bool    `json:"is_temporary"`
		Links       []uint32 `json:"links"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	opts := channel.UpdateOpts{}
	if body.Name != nil {
		opts.Name = body.Name
	}
	if body.Description != nil {
		opts.Description = body.Description
	}
	if body.Position != nil {
		opts.Position = body.Position
	}
	if body.MaxUsers != nil {
		opts.MaxUsers = body.MaxUsers
	}
	if body.Temporary != nil {
		opts.Temporary = body.Temporary
	}
	if body.Links != nil {
		opts.Links = body.Links
	}
	mgr := h.chanManager(uint(serverID))
	if !mgr.Update(uint32(chID), opts) {
		http.Error(w, `{"error":"channel not found or update failed"}`, http.StatusNotFound)
		return
	}
	if h.onChannelMutated != nil {
		if ch, ok := mgr.GetChannel(uint32(chID)); ok {
			h.onChannelMutated(uint(serverID), ch, uint32(chID), false)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteChannel removes a channel.
func (h *ServerHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	serverID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	chIDStr := chi.URLParam(r, "channelId")
	chID, err := strconv.ParseUint(chIDStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid channel id"}`, http.StatusBadRequest)
		return
	}
	mgr := h.chanManager(uint(serverID))
	if !mgr.Remove(uint32(chID)) {
		http.Error(w, `{"error":"channel not found, is root, or has children"}`, http.StatusBadRequest)
		return
	}
	if h.onChannelMutated != nil {
		h.onChannelMutated(uint(serverID), nil, uint32(chID), true)
	}
	w.WriteHeader(http.StatusNoContent)
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
		idStr := chi.URLParam(r, "id")
		serverID, _ := strconv.ParseUint(idStr, 10, 32)
		users = h.connectedUsers.ListConnected(uint(serverID), h.db)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// KickUser kicks a connected user.
func (h *ServerHandler) KickUser(w http.ResponseWriter, r *http.Request) {
	h.doUserAction(w, r, "kick", func(actioner ConnectedUserActioner, serverID uint, sessionID uint32, reason string) bool {
		return actioner.Kick(serverID, sessionID, reason)
	})
}

// MuteUser mutes or unmutes a connected user.
func (h *ServerHandler) MuteUser(w http.ResponseWriter, r *http.Request) {
	if h.userActioner == nil {
		http.Error(w, `{"error":"user actions not available"}`, http.StatusNotImplemented)
		return
	}
	serverIDStr := chi.URLParam(r, "id")
	sessionIDStr := chi.URLParam(r, "sessionId")
	serverID, err := strconv.ParseUint(serverIDStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	sessionID, err := strconv.ParseUint(sessionIDStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid session id"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		Mute bool `json:"mute"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if !h.userActioner.Mute(uint(serverID), uint32(sessionID), body.Mute) {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// BanUser bans and kicks a connected user.
func (h *ServerHandler) BanUser(w http.ResponseWriter, r *http.Request) {
	h.doUserAction(w, r, "ban", func(actioner ConnectedUserActioner, serverID uint, sessionID uint32, reason string) bool {
		return actioner.BanAndKick(serverID, sessionID, reason)
	})
}

func (h *ServerHandler) doUserAction(w http.ResponseWriter, r *http.Request, action string, fn func(ConnectedUserActioner, uint, uint32, string) bool) {
	if h.userActioner == nil {
		http.Error(w, `{"error":"user actions not available"}`, http.StatusNotImplemented)
		return
	}
	serverIDStr := chi.URLParam(r, "id")
	sessionIDStr := chi.URLParam(r, "sessionId")
	serverID, err := strconv.ParseUint(serverIDStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	sessionID, err := strconv.ParseUint(sessionIDStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid session id"}`, http.StatusBadRequest)
		return
	}
	reason := ""
	if r.Body != nil {
		var body struct {
			Reason string `json:"reason"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		reason = body.Reason
	}
	if !fn(h.userActioner, uint(serverID), uint32(sessionID), reason) {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Status returns overall server statistics.
func (h *ServerHandler) Status(w http.ResponseWriter, r *http.Request) {
	var serverCount, channelCount int64
	h.db.Model(&models.VirtualServer{}).Count(&serverCount)
	h.db.Model(&models.Channel{}).Count(&channelCount)
	meta, _ := config.LoadMetaConfig(h.db)
	restPort := 64730
	if meta != nil {
		restPort = meta.RESTPort
	}
	resp := map[string]interface{}{
		"status":      "ok",
		"servers":     serverCount,
		"channels":    channelCount,
		"mumble_port": h.metaMumblePort,
		"rest_port":   restPort,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
