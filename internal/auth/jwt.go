package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const algHS256 = "HS256"

var ErrTokenVersionMismatch = errors.New("token version mismatch")

// Config holds JWT configuration.
type Config struct {
	Issuer     string
	Audience   string
	ExpiryDays int
}

// GenerateSecret creates a 32-byte random secret for signing.
func GenerateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Sign creates a signed JWT for the user.
func Sign(cfg Config, userID uint, username, role string, tokenVersion int, secret string) (string, error) {
	now := time.Now()
	exp := now.Add(time.Duration(cfg.ExpiryDays) * 24 * time.Hour)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			Issuer:    cfg.Issuer,
			Audience:  jwt.ClaimStrings{cfg.Audience},
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		UserID:       userID,
		Username:     username,
		Role:         role,
		TokenVersion: tokenVersion,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// Validate parses and validates a JWT, returning claims if valid.
func Validate(tokenString, secret string, cfg Config) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != algHS256 {
			return nil, errors.New("invalid algorithm")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	if claims.Issuer != cfg.Issuer {
		return nil, errors.New("invalid issuer")
	}
	aud, _ := claims.GetAudience()
	if len(aud) == 0 || aud[0] != cfg.Audience {
		return nil, errors.New("invalid audience")
	}

	return claims, nil
}

// ParseUnverified parses a JWT without verifying the signature.
func ParseUnverified(tokenString string, claims *Claims) (*jwt.Token, error) {
	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(tokenString, claims)
	return token, err
}
