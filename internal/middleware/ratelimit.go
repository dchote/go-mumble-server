package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AuthRateLimit returns a middleware that limits requests per client IP for auth endpoints.
// Allows burst requests per window; excess receives 429 Too Many Requests.
type AuthRateLimit struct {
	mu       sync.Mutex
	byIP     map[string]*authRateEntry
	limit    int           // max requests per window
	window   time.Duration // window duration
	cleanup  time.Time     // next cleanup of old entries
	interval time.Duration // cleanup interval
}

type authRateEntry struct {
	count int
	start time.Time
}

// NewAuthRateLimit creates a rate limiter: limit requests per IP per window.
// Example: NewAuthRateLimit(10, time.Minute) = 10 requests per minute per IP.
func NewAuthRateLimit(limit int, window time.Duration) *AuthRateLimit {
	return &AuthRateLimit{
		byIP:     make(map[string]*authRateEntry),
		limit:    limit,
		window:   window,
		interval: window * 2,
		cleanup:  time.Now().Add(window * 2),
	}
}

func (rl *AuthRateLimit) getIP(r *http.Request) string {
	if x := r.Header.Get("X-Forwarded-For"); x != "" {
		x = strings.TrimSpace(x)
		if i := strings.Index(x, ","); i >= 0 {
			x = strings.TrimSpace(x[:i])
		}
		if x != "" {
			return x
		}
	}
	if r.RemoteAddr != "" {
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			return host
		}
		return r.RemoteAddr
	}
	return "unknown"
}

func (rl *AuthRateLimit) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.After(rl.cleanup) {
		for k, e := range rl.byIP {
			if now.Sub(e.start) > rl.window {
				delete(rl.byIP, k)
			}
		}
		rl.cleanup = now.Add(rl.interval)
	}

	e, ok := rl.byIP[ip]
	if !ok {
		rl.byIP[ip] = &authRateEntry{count: 1, start: now}
		return true
	}
	if now.Sub(e.start) > rl.window {
		e.count = 1
		e.start = now
		return true
	}
	if e.count >= rl.limit {
		return false
	}
	e.count++
	return true
}

// Handler returns middleware that returns 429 when the client IP exceeds the limit.
func (rl *AuthRateLimit) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rl.getIP(r)
		if !rl.allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"too many requests","code":"RATE_LIMITED"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
