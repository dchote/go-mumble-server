package middleware

import (
	"net/http"

	"github.com/dchote/go-mumble-server/internal/auth"
	"github.com/dchote/go-mumble-server/internal/httputil"
)

// RequireRole ensures the user has one of the allowed roles.
// Must be used after Auth middleware.
func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool)
	for _, r := range allowedRoles {
		allowed[r] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
			if !ok || claims == nil {
				httputil.WriteError(w, http.StatusUnauthorized, "authentication required", "UNAUTHORIZED", nil)
				return
			}

			if allowed["admin"] && claims.Role == "admin" {
				next.ServeHTTP(w, r)
				return
			}

			if allowed[claims.Role] {
				next.ServeHTTP(w, r)
				return
			}

			httputil.WriteError(w, http.StatusForbidden, "insufficient permissions", "FORBIDDEN", nil)
		})
	}
}
