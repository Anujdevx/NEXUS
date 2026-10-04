// Package http holds traffic-control's REST handlers: closures, breaks and the current road state.
package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"nexus/shared/auth"
	"nexus/shared/envelope"
	"nexus/shared/httpx"

	"github.com/nexus/traffic-control/internal/store"
)

type Publisher interface {
	Publish(ctx context.Context, env envelope.Envelope) error
}

type API struct {
	Store    *store.Store
	Pub      Publisher
	Secret   string
	Key      string
	GraphURL string
	HC       *http.Client
}

func (a *API) Register(app *httpx.App) {
	anyRole := auth.RequireOrKey(a.Secret, a.Key)
	field := auth.Require(a.Secret, auth.Controller, auth.Responder)
	ctl := auth.Require(a.Secret, auth.Controller)
	app.Handle("GET /api/v1/roads/closures", anyRole(a.closures))
	app.Handle("POST /api/v1/roads/closures/{id}/toggle", ctl(a.toggle))
	app.Handle("GET /api/v1/roads/breaks", anyRole(a.breaks))
	app.Handle("POST /api/v1/roads/breaks", field(a.addBreak))
	app.Handle("DELETE /api/v1/roads/breaks/{edgeId}", ctl(a.delBreak))
	app.Handle("GET /api/v1/roads/state", anyRole(a.state))
}

func (a *API) closures(w http.ResponseWriter, r *http.Request) {
	cs, err := a.Store.Closures(r.Context())
	if err != nil {
		httpx.Error(w, 500, "internal", err.Error())
		return
	}
	httpx.Items(w, cs)
}

func (a *API) state(w http.ResponseWriter, r *http.Request) {
	cs, err := a.Store.Closures(r.Context())
	if err != nil {
		httpx.Error(w, 500, "internal", err.Error())
		return
	}
	bs, err := a.Store.Breaks(r.Context())
	if err != nil {
		httpx.Error(w, 500, "internal", err.Error())
		return
	}
	active := []string{}
	for _, c := range cs {
		if c.Active {
			active = append(active, c.ID)
		}
	}
	httpx.JSON(w, 200, map[string]any{"closures": active, "breaks": bs})
}

func (a *API) toggle(w http.ResponseWriter, r *http.Request) {
	c, err := a.Store.SetClosure(r.Context(), r.PathValue("id"), nil)
	if err != nil {
		if err == store.ErrNotFound {
			httpx.Error(w, 404, "not_found", "no such closure")
			return
		}
		httpx.Error(w, 500, "internal", err.Error())
		return
	}
	a.PublishClosure(r.Context(), c, "marga")
	httpx.JSON(w, 200, c)
}

// PublishClosure announces a closure change (source marga, or the inferring module's name).
func (a *API) PublishClosure(ctx context.Context, c store.Closure, source string) {
	st := "open"
	if c.Active {
		st = c.Kind
	}
	env := envelope.New("road_segment", "passability", source, "traffic-control").WithStatus(st).WithGeo(c.Lat, c.Lng).
		WithPayload(map[string]any{"kind": "closure", "id": c.ID, "active": c.Active, "name": c.Name})
	_ = a.Pub.Publish(ctx, env)
}

func (a *API) breaks(w http.ResponseWriter, r *http.Request) {
	bs, err := a.Store.Breaks(r.Context())
	if err != nil {
		httpx.Error(w, 500, "internal", err.Error())
		return
	}
	httpx.Items(w, bs)
}

type breakIn struct {
	EdgeID *int     `json:"edgeId"`
	Lat    *float64 `json:"lat"`
	Lng    *float64 `json:"lng"`
	Reason string   `json:"reason"`
	Name   string   `json:"name"`
}

// addBreak records a reported broken road: by edge id (resolved to a point by graph-engine) or by lat/lng.
func (a *API) addBreak(w http.ResponseWriter, r *http.Request) {
	var in breakIn
	if !httpx.Decode(w, r, &in) {
		return
	}
	b := store.Break{Reason: in.Reason, Name: in.Name}
	if c := auth.FromContext(r.Context()); c != nil {
		b.CreatedBy = c.Role
	}
	switch {
	case in.EdgeID != nil:
		lat, lng, name, err := a.edgePoint(r.Context(), *in.EdgeID)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "bad_edge", err.Error())
			return
		}
		b.EdgeID, b.Lat, b.Lng = fmt.Sprint(*in.EdgeID), lat, lng
		if b.Name == "" {
			b.Name = name
		}
	case in.Lat != nil && in.Lng != nil:
		b.Lat, b.Lng = *in.Lat, *in.Lng
		b.EdgeID = fmt.Sprintf("p:%.5f,%.5f", b.Lat, b.Lng)
	default:
		httpx.Error(w, http.StatusBadRequest, "bad_request", "edgeId or lat and lng are required")
		return
	}
	if err := a.Store.AddBreak(r.Context(), b); err != nil {
		httpx.Error(w, 500, "internal", err.Error())
		return
	}
	a.PublishBreak(r.Context(), "add", b)
	httpx.JSON(w, http.StatusCreated, b)
}

func (a *API) delBreak(w http.ResponseWriter, r *http.Request) {
	id, _ := url.PathUnescape(r.PathValue("edgeId"))
	b, err := a.Store.RemoveBreak(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			httpx.Error(w, 404, "not_found", "no such break")
			return
		}
		httpx.Error(w, 500, "internal", err.Error())
		return
	}
	a.PublishBreak(r.Context(), "remove", *b)
	httpx.JSON(w, 200, map[string]string{"repaired": b.EdgeID})
}

func (a *API) PublishBreak(ctx context.Context, op string, b store.Break) {
	st := "closed"
	if op == "remove" {
		st = "open"
	}
	env := envelope.New("road_segment", "passability", "marga/report", "traffic-control").WithStatus(st).WithGeo(b.Lat, b.Lng).
		WithPayload(map[string]any{"kind": "break", "op": op, "lat": b.Lat, "lng": b.Lng, "name": b.Name, "edgeId": b.EdgeID})
	_ = a.Pub.Publish(ctx, env)
}

// edgePoint asks graph-engine for an edge's midpoint and name.
func (a *API) edgePoint(ctx context.Context, id int) (float64, float64, string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/graph/edges/%d", a.GraphURL, id), nil)
	req.Header.Set("X-Service-Key", a.Key)
	resp, err := a.HC.Do(req)
	if err != nil {
		return 0, 0, "", fmt.Errorf("graph-engine unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, 0, "", fmt.Errorf("no such edge %d", id)
	}
	var e struct {
		Lat  float64 `json:"lat"`
		Lng  float64 `json:"lng"`
		Name string  `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
		return 0, 0, "", err
	}
	return e.Lat, e.Lng, e.Name, nil
}
