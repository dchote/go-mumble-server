package auth

import (
	"github.com/golang-jwt/jwt/v5"
)

// ContextKey is the type for context keys.
type ContextKey string

// ClaimsKey is the context key for JWT claims.
const ClaimsKey ContextKey = "claims"

// Claims holds JWT claims.
type Claims struct {
	jwt.RegisteredClaims
	UserID       uint   `json:"userId"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	TokenVersion int    `json:"tokenVersion"`
}
