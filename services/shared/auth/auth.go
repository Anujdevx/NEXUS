// Package auth: JWT HS256 (shared secret across services), role middleware and the service-key check.
package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"nexus/shared/httpx"
)

const (
	Controller = "Controller"
	Responder  = "Responder"
	Citizen    = "Citizen"
)

type Claims struct {
	Email string `json:"email,omitempty"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

type ctxKey struct{}

func Sign(secret, sub, email, role string, ttl time.Duration) (string, time.Time, error) {
	exp := time.Now().Add(ttl)
	c := Claims{Email: email, Role: role, RegisteredClaims: jwt.RegisteredClaims{Subject: sub, ExpiresAt: jwt.NewNumericDate(exp), IssuedAt: jwt.NewNumericDate(time.Now())}}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secret))
	return s, exp, err
}

func Parse(secret, token string) (*Claims, error) {
	c := &Claims{}
	t, err := jwt.ParseWithClaims(token, c, func(t *jwt.Token) (any, error) { return []byte(secret), nil }, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	if !t.Valid {
		return nil, errors.New("invalid token")
	}
	return c, nil
}

// Bearer extracts the token from the Authorization header, or the ?token= query (WebSocket).
func Bearer(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

func FromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(ctxKey{}).(*Claims)
	return c
}

// Require returns a wrapper that demands a valid token whose role is in roles (any role if none given).
func Require(secret string, roles ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			tok := Bearer(r)
			if tok == "" {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
				return
			}
			c, err := Parse(secret, tok)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
				return
			}
			if len(roles) > 0 && c.Role != Controller && !has(roles, c.Role) { // Controller may do anything
				httpx.Error(w, http.StatusForbidden, "forbidden", "role "+c.Role+" may not do this")
				return
			}
			next(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, c)))
		}
	}
}

// ServiceKey guards machine-to-machine endpoints (telemetry ingest).
func ServiceKey(key string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Service-Key")), []byte(key)) != 1 {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized", "bad or missing X-Service-Key")
				return
			}
			next(w, r)
		}
	}
}

func has(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// RequireOrKey accepts either a valid bearer token (with one of roles) or the service key,
// for endpoints that other services call directly.
func RequireOrKey(secret, key string, roles ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		user := Require(secret, roles...)(next)
		return func(w http.ResponseWriter, r *http.Request) {
			if k := r.Header.Get("X-Service-Key"); k != "" && subtle.ConstantTimeCompare([]byte(k), []byte(key)) == 1 {
				next(w, r)
				return
			}
			user(w, r)
		}
	}
}
