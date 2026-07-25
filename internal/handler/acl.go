package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/dchote/go-mumble-server/internal/acl"
	"github.com/dchote/go-mumble-server/internal/database/models"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// ACLHandler handles ACL and group REST endpoints.
type ACLHandler struct {
	db          *gorm.DB
	OnACLChange func(serverID uint) // called after PUT to invalidate cache
}

// NewACLHandler creates an ACLHandler.
func NewACLHandler(db *gorm.DB, onACLChange func(serverID uint)) *ACLHandler {
	return &ACLHandler{db: db, OnACLChange: onACLChange}
}

// ACLResponse is the GET response for channel ACL.
type ACLResponse struct {
	Groups []ACLGroupResp `json:"groups"`
	ACLs   []ACLEntryResp `json:"acls"`
}

type ACLGroupResp struct {
	ID            uint     `json:"id"`
	Name          string   `json:"name"`
	Inherit       bool     `json:"inherit"`
	Inheritable   bool     `json:"inheritable"`
	AddUserIDs    []uint32 `json:"add_user_ids"`
	RemoveUserIDs []uint32 `json:"remove_user_ids"`
}

type ACLEntryResp struct {
	ID          uint   `json:"id"`
	Priority    int    `json:"priority"`
	ApplyHere   bool   `json:"apply_here"`
	ApplySubs   bool   `json:"apply_subs"`
	UserID      *int32 `json:"user_id,omitempty"`
	GroupName   string `json:"group_name"`
	AccessToken string `json:"access_token"`
	EvalHere    bool   `json:"eval_here"`
	Invert      bool   `json:"invert"`
	Grant       uint32 `json:"grant"`
	Deny        uint32 `json:"deny"`
}

// Get returns ACL and groups for a channel.
func (h *ACLHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	serverID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	chIDStr := chi.URLParam(r, "channelId")
	channelID, err := strconv.ParseUint(chIDStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid channel id"}`, http.StatusBadRequest)
		return
	}
	var groups []models.ChannelGroup
	if err := h.db.Where("server_id = ? AND channel_id = ?", serverID, channelID).Find(&groups).Error; err != nil {
		http.Error(w, `{"error":"failed to load groups"}`, http.StatusInternalServerError)
		return
	}
	var acls []models.ChannelACL
	if err := h.db.Where("server_id = ? AND channel_id = ?", serverID, channelID).Order("priority, id").Find(&acls).Error; err != nil {
		http.Error(w, `{"error":"failed to load ACLs"}`, http.StatusInternalServerError)
		return
	}
	gr := make([]ACLGroupResp, 0, len(groups))
	for _, g := range groups {
		gr = append(gr, ACLGroupResp{
			ID: g.ID, Name: g.Name, Inherit: g.Inherit, Inheritable: g.Inheritable,
			AddUserIDs: g.AddUserIDs, RemoveUserIDs: g.RemoveUserIDs,
		})
	}
	ar := make([]ACLEntryResp, 0, len(acls))
	for _, a := range acls {
		ar = append(ar, ACLEntryResp{
			ID: a.ID, Priority: a.Priority, ApplyHere: a.ApplyHere, ApplySubs: a.ApplySubs,
			UserID: a.UserID, GroupName: a.GroupName, AccessToken: a.AccessToken,
			EvalHere: a.EvalHere, Invert: a.Invert, Grant: a.Grant, Deny: a.Deny,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ACLResponse{Groups: gr, ACLs: ar})
}

// Put replaces ACL and groups for a channel.
func (h *ACLHandler) Put(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	serverID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid server id"}`, http.StatusBadRequest)
		return
	}
	chIDStr := chi.URLParam(r, "channelId")
	channelID, err := strconv.ParseUint(chIDStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid channel id"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		Groups []struct {
			Name          string   `json:"name"`
			Inherit       bool     `json:"inherit"`
			Inheritable   bool     `json:"inheritable"`
			AddUserIDs    []uint32 `json:"add_user_ids"`
			RemoveUserIDs []uint32 `json:"remove_user_ids"`
		} `json:"groups"`
		ACLs []struct {
			Priority    int    `json:"priority"`
			ApplyHere   bool   `json:"apply_here"`
			ApplySubs   bool   `json:"apply_subs"`
			UserID      *int32 `json:"user_id"`
			GroupName   string `json:"group_name"`
			AccessToken string `json:"access_token"`
			EvalHere    bool   `json:"eval_here"`
			Invert      bool   `json:"invert"`
			Grant       uint32 `json:"grant"`
			Deny        uint32 `json:"deny"`
		} `json:"acls"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("server_id = ? AND channel_id = ?", serverID, channelID).Delete(&models.ChannelGroup{}).Error; err != nil {
			return err
		}
		if err := tx.Where("server_id = ? AND channel_id = ?", serverID, channelID).Delete(&models.ChannelACL{}).Error; err != nil {
			return err
		}
		for _, g := range body.Groups {
			if g.Name == "" {
				continue
			}
			row := models.ChannelGroup{
				ServerID:      uint(serverID),
				ChannelID:     uint(channelID),
				Name:          g.Name,
				Inherit:       g.Inherit,
				Inheritable:   g.Inheritable,
				AddUserIDs:    models.Uint32Slice(g.AddUserIDs),
				RemoveUserIDs: models.Uint32Slice(g.RemoveUserIDs),
			}
			if err := acl.CreateGroup(tx, &row); err != nil {
				return err
			}
		}
		for _, a := range body.ACLs {
			row := models.ChannelACL{
				ServerID:    uint(serverID),
				ChannelID:   uint(channelID),
				Priority:    a.Priority,
				ApplyHere:   a.ApplyHere,
				ApplySubs:   a.ApplySubs,
				UserID:      a.UserID,
				GroupName:   a.GroupName,
				AccessToken: a.AccessToken,
				EvalHere:    a.EvalHere,
				Invert:      a.Invert,
				Grant:       a.Grant,
				Deny:        a.Deny,
			}
			if err := acl.CreateACL(tx, &row); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		http.Error(w, `{"error":"failed to save ACL"}`, http.StatusInternalServerError)
		return
	}
	if h.OnACLChange != nil {
		h.OnACLChange(uint(serverID))
	}
	w.WriteHeader(http.StatusNoContent)
}
