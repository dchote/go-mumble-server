package rest

import (
	"bytes"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/dchote/go-mumble-server/api"
	"github.com/dchote/go-mumble-server/internal/channel"
	"github.com/dchote/go-mumble-server/internal/config"
	"github.com/dchote/go-mumble-server/internal/handler"
	"github.com/dchote/go-mumble-server/internal/middleware"
	"github.com/dchote/go-mumble-server/internal/service"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// ConnectedUser is a connected Mumble user for the REST API.
type ConnectedUser struct {
	SessionID       uint32  `json:"session_id"`
	UserID          uint32  `json:"user_id"`
	Name            string  `json:"name"`
	ChannelID       uint32  `json:"channel_id"`
	Address         string  `json:"address"`
	Ping            float32 `json:"ping"`
	CertificateHash string  `json:"certificate_hash,omitempty"`   // SHA-1 hex of client cert; empty if none
	CryptoMode      string  `json:"crypto_mode,omitempty"`        // Negotiated UDP crypto tier: lite, legacy, secure
	VoiceTransport  string  `json:"voice_transport,omitempty"`    // udp or tcp — whether client uses native UDP or TCP tunnel for voice
	SelfMute        bool    `json:"self_mute"`
	SelfDeaf        bool    `json:"self_deaf"`
	Mute            bool    `json:"mute"`
	Deaf            bool    `json:"deaf"`
	IsAdmin         bool    `json:"is_admin"`
}

// GetChannelManager returns the channel manager for a server ID, or nil.
// When nil, channel CRUD uses DB only (no live Mumble sync for server 1).
type GetChannelManager func(serverID uint) *channel.Manager

// OnACLChange is called when ACLs are modified via REST (for cache invalidation).
type OnACLChange func(serverID uint)

// OnBanChange is called when bans are created or deleted via REST (for Mumble cache invalidation).
type OnBanChange func(serverID uint)

// RouterWithMumble sets up the REST API with optional Mumble connected-user listing.
func RouterWithMumble(db *gorm.DB, cfg *config.Config, feFS fs.FS, userLister handler.ConnectedUserLister, userActioner handler.ConnectedUserActioner, channelCrypto handler.ChannelCryptoLister, getChanMgr GetChannelManager, onACLChange OnACLChange, onBanChange OnBanChange, onChannelMutated handler.OnChannelMutated, onConfigChange handler.OnConfigChange) http.Handler {
	userSvc := service.NewUserService(db, cfg)
	authHandler := handler.NewAuthHandler(userSvc, db, cfg)
	userHandler := handler.NewUserHandler(userSvc)
	serverHandler := handler.NewServerHandler(db, cfg, userLister, userActioner, channelCrypto, (func(serverID uint) *channel.Manager)(getChanMgr), onChannelMutated, onConfigChange)
	banHandler := handler.NewBanHandler(db, (handler.OnBanChange)(onBanChange))
	aclHandler := handler.NewACLHandler(db, onACLChange)
	regUserHandler := handler.NewRegisteredUserHandler(db)

	r := chi.NewRouter()
	r.Use(middleware.Logging)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Get("/api/v1/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Write(api.OpenAPI)
	})

	r.Get("/docs", swaggerUIHandler)

	authRateLimit := middleware.NewAuthRateLimit(20, time.Minute)
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/auth/status", authHandler.Status)
		r.Group(func(r chi.Router) {
			r.Use(authRateLimit.Handler)
			r.Post("/auth/register", authHandler.Register)
			r.Post("/auth/login", authHandler.Login)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(true, db, cfg, userSvc))
			r.Get("/status", serverHandler.Status)
			r.Get("/servers", serverHandler.List)
			r.Get("/servers/{id}", serverHandler.Get)
			r.Get("/servers/{id}/config", serverHandler.GetConfig)
			r.Get("/servers/{id}/channels", serverHandler.GetChannels)
			r.Get("/servers/{id}/users", serverHandler.GetUsers)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(true, db, cfg, userSvc))
			r.Use(middleware.RequireRole("admin"))
			r.Post("/servers", serverHandler.Create)
			r.Patch("/servers/{id}", serverHandler.Update)
			r.Delete("/servers/{id}", serverHandler.Delete)
			r.Patch("/servers/{id}/config", serverHandler.UpdateConfig)
			r.Post("/servers/{id}/channels", serverHandler.CreateChannel)
			r.Patch("/servers/{id}/channels/{channelId}", serverHandler.UpdateChannel)
			r.Delete("/servers/{id}/channels/{channelId}", serverHandler.DeleteChannel)
			r.Get("/servers/{id}/channels/{channelId}/acl", aclHandler.Get)
			r.Put("/servers/{id}/channels/{channelId}/acl", aclHandler.Put)
			r.Post("/servers/{id}/users/{sessionId}/kick", serverHandler.KickUser)
			r.Post("/servers/{id}/users/{sessionId}/mute", serverHandler.MuteUser)
			r.Post("/servers/{id}/users/{sessionId}/ban", serverHandler.BanUser)
			r.Get("/servers/{id}/bans", banHandler.List)
			r.Post("/servers/{id}/bans", banHandler.Create)
			r.Delete("/servers/{id}/bans/{banId}", banHandler.Delete)
			r.Get("/servers/{id}/registered-users", regUserHandler.List)
			r.Post("/servers/{id}/registered-users", regUserHandler.Create)
			r.Patch("/servers/{id}/registered-users/{userId}", regUserHandler.Update)
			r.Delete("/servers/{id}/registered-users/{userId}", regUserHandler.Delete)
			r.Get("/users", userHandler.List)
			r.Patch("/users/{id}", userHandler.Update)
			r.Delete("/users/{id}", userHandler.Delete)
			r.Get("/meta/config", serverHandler.GetMetaConfig)
			r.Patch("/meta/config", serverHandler.UpdateMetaConfig)
		})
	})

	if feFS != nil {
		spa := spaHandler(feFS)
		r.Get("/", spa)
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			spa(w, r)
		})
	} else {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("go-mumble-server. Start with -frontend-embed=false and use API at /api/v1, /health"))
		})
	}

	return r
}

const swaggerUIHTML = `<!DOCTYPE html>
<html>
<head>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: '/api/v1/openapi.yaml',
      dom_id: '#swagger-ui',
    });
  </script>
</body>
</html>
`

func swaggerUIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(swaggerUIHTML))
}

func isAssetPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".mjs", ".css", ".woff2", ".woff", ".ttf", ".ico", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".json", ".map":
		return true
	}
	return strings.HasPrefix(path, "assets/")
}

// appBaseHref returns the base href for the SPA so relative asset URLs resolve correctly
// (e.g. on reload of /servers/1 or under HA ingress /api/hassio_ingress/<token>/...).
func appBaseHref(requestPath string) string {
	path := strings.TrimPrefix(requestPath, "/")
	parts := strings.Split(path, "/")
	if len(parts) >= 3 && parts[0] == "api" && parts[1] == "hassio_ingress" && parts[2] != "" {
		return "/" + strings.Join(parts[0:3], "/") + "/"
	}
	return "/"
}

func spaHandler(feFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if feFS == nil {
			http.NotFound(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		_, statErr := fs.Stat(feFS, path)
		if statErr != nil {
			if isAssetPath(path) {
				http.NotFound(w, r)
				return
			}
			path = "index.html"
		}
		f, err := feFS.Open(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		stat, err := f.Stat()
		if err != nil || stat.IsDir() {
			http.NotFound(w, r)
			return
		}
		if ct := mime.TypeByExtension(filepath.Ext(path)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		if path == "index.html" {
			// Inject <base href="..."> so relative asset URLs (./assets/...) resolve correctly
			// on direct load or reload of routes like /servers/1 or under HA ingress.
			body, err := io.ReadAll(f)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			base := appBaseHref(r.URL.Path)
			baseTag := []byte("<base href=\"" + base + "\">")
			head := []byte("<head>")
			idx := bytes.Index(body, head)
			if idx >= 0 {
				body = bytes.Join([][]byte{body[:idx+len(head)], baseTag, body[idx+len(head):]}, nil)
			}
			w.Write(body)
			return
		}
		if rs, ok := f.(io.ReadSeeker); ok {
			http.ServeContent(w, r, path, stat.ModTime(), rs)
		} else {
			io.Copy(w, f)
		}
	}
}
