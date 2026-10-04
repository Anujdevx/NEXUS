// Package http holds notification-gateway's REST handlers and the alert publisher.
package http

import (
	"context"
	"net/http"

	"nexus/shared/auth"
	"nexus/shared/envelope"
	"nexus/shared/httpx"

	"github.com/nexus/notification-gateway/internal/domain"
	"github.com/nexus/notification-gateway/internal/store"
)

type Publisher interface {
	Publish(ctx context.Context, env envelope.Envelope) error
}

type API struct {
	Store  *store.Store
	Pub    Publisher
	Secret string
	Key    string
}

func (a *API) Register(app *httpx.App) {
	app.Handle("POST /api/v1/alerts", auth.Require(a.Secret, auth.Controller)(a.issue))
	app.Handle("GET /api/v1/alerts", auth.RequireOrKey(a.Secret, a.Key)(a.list))
}

func (a *API) issue(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Area     string `json:"area"`
		Severity string `json:"severity"`
		TextEN   string `json:"text_en"`
		TextHI   string `json:"text_hi"`
		Event    string `json:"event"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	al, err := domain.NewAlert(in.Area, in.Severity, in.TextEN, in.TextHI, auth.FromContext(r.Context()).Role, "manual", in.Event)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := a.Publish(r.Context(), al); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, al)
}

// Publish stores an alert and puts public_alert.cap_alert on the bus.
func (a *API) Publish(ctx context.Context, al domain.Alert) error {
	if err := a.Store.Save(ctx, al); err != nil {
		return err
	}
	env := envelope.New("public_alert", "cap_alert", "purvasuchana", "notification-gateway").WithStatus(al.Severity).
		WithPayload(map[string]any{"id": al.ID, "area": al.Area, "severity": al.Severity, "message": al.TextEN, "message_hi": al.TextHI, "by": al.IssuedBy, "at": al.IssuedAt, "auto": al.Source != "manual", "cap": al.CAP})
	return a.Pub.Publish(ctx, env)
}

func (a *API) list(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.List(r.Context(), 200)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.Items(w, list)
}
