package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/dchote/go-mumble-server/internal/auth"
	"github.com/dchote/go-mumble-server/internal/config"
	"github.com/dchote/go-mumble-server/internal/httputil"
	"github.com/dchote/go-mumble-server/internal/service"
	"gorm.io/gorm"
)

// Auth extracts and validates JWT, attaching claims to context.
func Auth(required bool, db *gorm.DB, cfg *config.Config, userSvc *service.UserService) func(http.Handler) http.Handler {
	jwtCfg := auth.Config{
		Issuer:     cfg.JWTIssuer,
		Audience:   cfg.JWTAudience,
		ExpiryDays: cfg.JWTExpiryDays,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				if required {
					httputil.WriteError(w, http.StatusUnauthorized, "missing authorization", "UNAUTHORIZED", nil)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				if required {
					httputil.WriteError(w, http.StatusUnauthorized, "invalid authorization header", "UNAUTHORIZED", nil)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			claims, err := ParseAndValidateToken(db, parts[1], jwtCfg)
			if err != nil {
				if required {
					httputil.WriteError(w, http.StatusUnauthorized, "invalid or expired token", "UNAUTHORIZED", nil)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			if userSvc != nil {
				go userSvc.UpdateLastActivity(claims.UserID)
			}
			ctx := context.WithValue(r.Context(), auth.ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ParseAndValidateToken parses and validates a JWT.
func ParseAndValidateToken(db *gorm.DB, tokenString string, cfg auth.Config) (*auth.Claims, error) {
	claims := &auth.Claims{}
	_, err := auth.ParseUnverified(tokenString, claims)
	if err != nil {
		return nil, err
	}

	user, err := service.GetUserForAuth(db, claims.UserID)
	if err != nil {
		return nil, err
	}

	validClaims, err := auth.Validate(tokenString, user.JWTSecret, cfg)
	if err != nil {
		return nil, err
	}

	if validClaims.TokenVersion != user.TokenVersion {
		return nil, auth.ErrTokenVersionMismatch
	}

	return validClaims, nil
}
