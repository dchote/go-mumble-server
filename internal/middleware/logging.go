package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// Logging logs HTTP requests (method, path, status, duration).
// Skips static assets to reduce noise.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		if shouldLogRequest(r.URL.Path) {
			slog.Info("REST request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"remote", r.RemoteAddr,
			)
		}
	})
}

func shouldLogRequest(path string) bool {
	if path == "/" || path == "/index.html" {
		return false
	}
	if path == "/health" || path == "/docs" || (len(path) >= 4 && path[:4] == "/api") {
		return true
	}
	return !isStaticAssetPath(path)
}

func isStaticAssetPath(path string) bool {
	if len(path) >= 8 && path[:8] == "/assets/" {
		return true
	}
	for _, ext := range []string{".js", ".css", ".ico", ".png", ".jpg", ".svg", ".woff2", ".woff", ".ttf", ".map", ".json"} {
		if len(path) >= len(ext) && path[len(path)-len(ext):] == ext {
			return true
		}
	}
	return false
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
