// Package http holds environmental-monitor's REST handlers.
package http

import (
	"context"
	"fmt"
	"net/http"

	"nexus/shared/auth"
	"nexus/shared/envelope"
	"nexus/shared/httpx"

	"github.com/nexus/environmental-monitor/internal/domain"
	"github.com/nexus/environmental-monitor/internal/store"
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
	anyRole := auth.RequireOrKey(a.Secret, a.Key, auth.Controller, auth.Responder) // operational data: not for citizens
	ctl := auth.Require(a.Secret, auth.Controller)
	list := func(f func(context.Context) ([]map[string]any, error), source string) http.HandlerFunc {
		return anyRole(func(w http.ResponseWriter, r *http.Request) {
			rows, err := f(r.Context())
			if err != nil {
				httpx.Error(w, 500, "internal", err.Error())
				return
			}
			httpx.JSON(w, 200, map[string]any{"items": rows, "source": source})
		})
	}
	app.Handle("GET /api/v1/environment/rain", list(a.Store.Rain, "documented + live readings (see each row's source)"))
	app.Handle("GET /api/v1/environment/rivers", list(a.Store.Rivers, "documented + live readings (see each row's source)"))
	app.Handle("GET /api/v1/environment/flood-zones", list(a.Store.FloodZones, "documented"))
	app.Handle("GET /api/v1/environment/landslides", list(a.Store.Landslides, "documented"))
	app.Handle("GET /api/v1/environment/zoning", list(a.Store.Zoning, "ajg2023"))
	app.Handle("GET /api/v1/environment/flood-stage", anyRole(a.getStage))
	app.Handle("POST /api/v1/environment/flood-stage", ctl(a.setStage))
}

func (a *API) getStage(w http.ResponseWriter, r *http.Request) {
	st, src, err := a.Store.FloodStage(r.Context())
	if err != nil {
		httpx.Error(w, 500, "internal", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"stage": st, "source": src})
}

func (a *API) setStage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Stage float64 `json:"stage"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	stage := domain.ClampStage(in.Stage)
	if err := a.Store.SetFloodStage(r.Context(), stage, "purvasuchana"); err != nil {
		httpx.Error(w, 500, "internal", err.Error())
		return
	}
	a.PublishStage(r.Context(), stage, "purvasuchana")
	httpx.JSON(w, 200, map[string]any{"stage": stage})
}

// PublishStage announces the flood stage (illustrative, never a hydraulic model: confidence 0.5).
func (a *API) PublishStage(ctx context.Context, stage float64, source string) {
	env := envelope.New("flood", "stage", source, "environmental-monitor").WithStatus(fmt.Sprintf("%d%%", int(stage+0.5))).WithConfidence(0.5).
		WithPayload(map[string]any{"stage": stage})
	_ = a.Pub.Publish(ctx, env)
}
