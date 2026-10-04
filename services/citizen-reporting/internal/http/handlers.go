// Package http holds citizen-reporting's REST handlers: SOS, incidents and hazard reports.
package http

import (
	"context"
	"net/http"
	"time"

	"nexus/shared/auth"
	"nexus/shared/envelope"
	"nexus/shared/httpx"
	"nexus/shared/ratelimit"

	"github.com/nexus/citizen-reporting/internal/domain"
	"github.com/nexus/citizen-reporting/internal/store"
)

type Publisher interface {
	Publish(ctx context.Context, env envelope.Envelope) error
}

type API struct {
	Store   *store.Store
	Pub     Publisher
	Limiter *ratelimit.Limiter
	Secret  string
}

func (a *API) Register(app *httpx.App) {
	anyRole := auth.Require(a.Secret)
	field := auth.Require(a.Secret, auth.Controller, auth.Responder)
	app.Handle("POST /api/v1/sos", anyRole(a.sos))
	app.Handle("GET /api/v1/incidents", field(a.incidents))
	app.Handle("PATCH /api/v1/incidents/{id}", field(a.patchIncident))
	app.Handle("POST /api/v1/reports", anyRole(a.report))
	app.Handle("GET /api/v1/reports", field(a.reports))
}

// sos accepts a request from any role. It stores the SOS and its incident, then publishes sos.request;
// dispatch ranks hospitals and assigns an ambulance, and the assignment comes back over the live stream.
func (a *API) sos(w http.ResponseWriter, r *http.Request) {
	if !a.Limiter.Allow(r.Context(), "sos:"+httpx.ClientIP(r), 30, time.Minute) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many SOS requests from this address")
		return
	}
	var in domain.SOS
	if !httpx.Decode(w, r, &in) {
		return
	}
	if err := in.Normalize(); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if in.Client == "" {
		in.Client = r.Header.Get("X-Client-Id")
	}
	inc := domain.IncidentFromSOS(in)
	if err := a.Store.SaveSOS(r.Context(), in, inc); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	payload := map[string]any{
		"id": in.ID, "place": in.Place, "placeName": in.PlaceName, "hazard": in.Hazard, "people": in.People, "injured": in.Injured,
		"lat": in.Lat, "lng": in.Lng, "node": in.Node, "via": in.Via, "raised": in.Raised, "client": in.Client, "incident": inc,
	}
	env := envelope.New("sos", "request", "ahvana", "citizen-reporting").WithGeo(in.Lat, in.Lng).WithStatus(in.Via).
		WithCapacity(map[string]any{"people": in.People}).WithPayload(payload)
	if err := a.Pub.Publish(r.Context(), env); err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "bus_unavailable", err.Error())
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]any{"id": in.ID, "status": "received", "envelope": env.ID})
}

func (a *API) incidents(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.Incidents(r.Context(), 200)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.Items(w, list)
}

var validStatus = map[string]string{"acknowledged": "Acknowledged", "closed": "Closed", "open": "Open"}

func (a *API) patchIncident(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Status string `json:"status"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	st := in.Status
	key := ""
	for k, v := range validStatus {
		if st == v || st == k {
			st, key = v, k
		}
	}
	if key == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "status must be Open, Acknowledged or Closed")
		return
	}
	id := r.PathValue("id")
	ok, err := a.Store.SetStatus(r.Context(), id, st, nil)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !ok {
		httpx.Error(w, http.StatusNotFound, "not_found", "no such incident")
		return
	}
	_ = a.Store.AddLog(r.Context(), id, st)
	inc, _ := a.Store.Incident(r.Context(), id)
	unit := ""
	if inc != nil && inc.Unit != nil {
		unit = *inc.Unit
	}
	env := envelope.New("incident", key, "sutradhara", "citizen-reporting").WithStatus(st).WithPayload(map[string]any{"id": id, "unit": unit, "status": st})
	if inc != nil {
		env = env.WithGeo(inc.Lat, inc.Lng)
	}
	_ = a.Pub.Publish(r.Context(), env)
	httpx.JSON(w, 200, inc)
}

func (a *API) report(w http.ResponseWriter, r *http.Request) {
	var in store.Report
	if !httpx.Decode(w, r, &in) {
		return
	}
	if in.Type == "" || (in.Lat == 0 && in.Lng == 0) {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "type, lat and lng are required")
		return
	}
	if c := auth.FromContext(r.Context()); c != nil {
		in.UserID = c.Subject
	}
	saved, err := a.Store.SaveReport(r.Context(), in)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	env := envelope.New("hazard_report", "created", "ahvana", "citizen-reporting").WithGeo(saved.Lat, saved.Lng).WithStatus(saved.Type).WithPayload(saved)
	_ = a.Pub.Publish(r.Context(), env)
	httpx.JSON(w, http.StatusCreated, saved)
}

func (a *API) reports(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.Reports(r.Context(), 200)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.Items(w, list)
}
