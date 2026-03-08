package rest

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/dchote/go-mumble-server/api"
	"github.com/dchote/go-mumble-server/internal/config"
	"github.com/dchote/go-mumble-server/internal/handler"
	"github.com/dchote/go-mumble-server/internal/middleware"
	"github.com/dchote/go-mumble-server/internal/service"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// ConnectedUser is a connected Mumble user for the REST API.
type ConnectedUser struct {
	SessionID uint32 `json:"session_id"`
	UserID    uint32 `json:"user_id"`
	Name      string `json:"name"`
	ChannelID uint32 `json:"channel_id"`
}

// ConnectedUserLister lists connected Mumble users for REST.
type ConnectedUserLister interface {
	ListConnected() interface{}
}

// Router sets up the REST API and optionally serves the embedded SPA.
func Router(db *gorm.DB, cfg *config.Config, feFS fs.FS) http.Handler {
	return RouterWithMumble(db, cfg, feFS, nil)
}

// RouterWithMumble sets up the REST API with optional Mumble connected-user listing.
func RouterWithMumble(db *gorm.DB, cfg *config.Config, feFS fs.FS, userLister ConnectedUserLister) http.Handler {
	userSvc := service.NewUserService(db, cfg)
	authHandler := handler.NewAuthHandler(userSvc, db, cfg)
	userHandler := handler.NewUserHandler(userSvc)
	serverHandler := handler.NewServerHandler(db, cfg, userLister)

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

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/auth/status", authHandler.Status)
		r.Post("/auth/register", authHandler.Register)
		r.Post("/auth/login", authHandler.Login)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(true, db, cfg, userSvc))
			r.Get("/status", serverHandler.Status)
			r.Get("/servers", serverHandler.List)
			r.Get("/servers/{id}/channels", serverHandler.GetChannels)
			r.Get("/servers/{id}/users", serverHandler.GetUsers)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(true, db, cfg, userSvc))
			r.Use(middleware.RequireRole("admin"))
			r.Get("/users", userHandler.List)
			r.Patch("/users/{id}", userHandler.Update)
			r.Delete("/users/{id}", userHandler.Delete)
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
		if rs, ok := f.(io.ReadSeeker); ok {
			http.ServeContent(w, r, path, stat.ModTime(), rs)
		} else {
			io.Copy(w, f)
		}
	}
}
