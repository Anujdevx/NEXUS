// Package http holds the identity-access REST handlers.
package http

import (
	"net/http"
	"strings"
	"time"

	"nexus/shared/auth"
	"nexus/shared/bus"
	"nexus/shared/envelope"
	"nexus/shared/httpx"
	"nexus/shared/ratelimit"

	"github.com/nexus/identity-access/internal/domain"
	"github.com/nexus/identity-access/internal/store"
)

type API struct {
	Store    *store.Store
	Bus      *bus.Bus
	Limiter  *ratelimit.Limiter
	Secret   string
	DemoMode bool
	TTL      time.Duration
}

func (a *API) Register(app *httpx.App) {
	app.Handle("POST /api/v1/auth/login", a.login)
	app.Handle("POST /api/v1/auth/demo-token", a.demoToken)
	app.Handle("GET /api/v1/auth/config", a.config)
	app.Handle("GET /api/v1/auth/me", auth.Require(a.Secret)(a.me))
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	if !a.Limiter.Allow(r.Context(), "login:"+httpx.ClientIP(r), 10, time.Minute) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many login attempts, wait a minute")
		return
	}
	var in struct{ Email, Password string }
	if !httpx.Decode(w, r, &in) {
		return
	}
	u, err := a.Store.ByEmail(r.Context(), strings.TrimSpace(in.Email))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "lookup failed")
		return
	}
	if u == nil || !domain.CheckPassword(in.Password, u.Hash) {
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "email or password is wrong")
		return
	}
	tok, exp, err := auth.Sign(a.Secret, u.ID, u.Email, u.Role, a.TTL)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "could not sign token")
		return
	}
	_ = a.Bus.Publish(r.Context(), envelope.New("user", "login", "identity-access", "identity-access").WithStatus(u.Role).WithPayload(map[string]string{"id": u.ID, "role": u.Role}))
	httpx.JSON(w, 200, map[string]any{"token": tok, "role": u.Role, "expiresAt": exp.UTC().Format(time.RFC3339)})
}

// demoToken keeps role switching instant in the console: no login screen, only when DEMO_MODE=true.
func (a *API) demoToken(w http.ResponseWriter, r *http.Request) {
	if !a.DemoMode {
		httpx.Error(w, http.StatusForbidden, "demo_disabled", "demo tokens are disabled (DEMO_MODE=false)")
		return
	}
	var in struct{ Role string }
	if !httpx.Decode(w, r, &in) {
		return
	}
	if !domain.ValidRole(in.Role) {
		httpx.Error(w, http.StatusBadRequest, "bad_role", "role must be Controller, Responder or Citizen")
		return
	}
	email := strings.ToLower(in.Role) + "@nexus.local"
	tok, exp, err := auth.Sign(a.Secret, "demo-"+strings.ToLower(in.Role), email, in.Role, a.TTL)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "could not sign token")
		return
	}
	httpx.JSON(w, 200, map[string]any{"token": tok, "role": in.Role, "expiresAt": exp.UTC().Format(time.RFC3339)})
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	httpx.JSON(w, 200, map[string]string{"id": c.Subject, "email": c.Email, "role": c.Role})
}

// config tells the console whether it may mint demo tokens (no login screen) or must ask for a login.
func (a *API) config(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, 200, map[string]any{"demo": a.DemoMode, "roles": []string{auth.Controller, auth.Responder, auth.Citizen}})
}
