// Package http holds the dispatch REST handlers: hospitals and ranking, units, shelters, pharmacies, approvals.
package http

import (
	"errors"
	"net/http"
	"sort"
	"strconv"

	"nexus/shared/auth"
	"nexus/shared/httpx"

	"github.com/nexus/emergency-dispatch/internal/dispatch"
	"github.com/nexus/emergency-dispatch/internal/domain"
	"github.com/nexus/emergency-dispatch/internal/store"
)

type API struct {
	D      *dispatch.Dispatcher
	Secret string
	Key    string
	Stubs  map[string]map[string]any // module stub envelopes for tier-3 modules
}

func (a *API) Register(app *httpx.App) {
	anyRole := auth.RequireOrKey(a.Secret, a.Key, auth.Controller, auth.Responder) // operational data: not for citizens
	ctl := auth.Require(a.Secret, auth.Controller)
	field := auth.Require(a.Secret, auth.Controller, auth.Responder)
	app.Handle("GET /api/v1/hospitals", anyRole(a.hospitals))
	app.Handle("POST /api/v1/hospitals/rank", anyRole(a.rank))
	app.Handle("PATCH /api/v1/hospitals/{id}", ctl(a.patchHospital))
	app.Handle("GET /api/v1/units", anyRole(a.units))
	app.Handle("PATCH /api/v1/units/{id}", field(a.patchUnit))
	app.Handle("GET /api/v1/shelters", anyRole(a.shelters))
	app.Handle("PATCH /api/v1/shelters/{id}", ctl(a.patchShelter))
	app.Handle("GET /api/v1/pharmacies", anyRole(a.pharmacies))
	app.Handle("PATCH /api/v1/pharmacies/{id}/stock", ctl(a.patchStock))
	app.Handle("POST /api/v1/dispatch/approve", ctl(a.approve))
	app.Handle("GET /api/v1/dispatch/assignments", anyRole(a.assignments))
	app.Handle("GET /api/v1/dispatch/registry", a.stub("abhaya"))
	app.Handle("GET /api/v1/dispatch/logistics", a.stub("sambhar"))
}

func fail(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "no such record")
		return
	}
	httpx.Error(w, http.StatusInternalServerError, "internal", err.Error())
}

func (a *API) hospitals(w http.ResponseWriter, r *http.Request) {
	hs, err := a.D.Store.Hospitals(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	httpx.Items(w, hs)
}

func (a *API) rank(w http.ResponseWriter, r *http.Request) {
	var in struct {
		At   domain.Point `json:"at"`
		Need string       `json:"need"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	if in.Need == "" {
		in.Need = "General"
	}
	rows, est, err := a.D.Rank(r.Context(), in.At, in.Need)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"items": rows, "estimator": est})
}

func (a *API) patchHospital(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Free    *int  `json:"free"`
		Inbound *int  `json:"inbound"`
		Divert  *bool `json:"divert"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	h, err := a.D.Store.PatchHospital(r.Context(), r.PathValue("id"), in.Free, in.Inbound, in.Divert)
	if err != nil {
		fail(w, err)
		return
	}
	a.D.PublishHospital(r.Context(), h, "patch")
	httpx.JSON(w, 200, h)
}

func (a *API) units(w http.ResponseWriter, r *http.Request) {
	us, err := a.D.Store.Units(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	httpx.Items(w, us)
}

func (a *API) patchUnit(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Status string  `json:"status"`
		Task   *string `json:"task"`
		Step   *string `json:"step"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	if in.Status == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "status is required")
		return
	}
	u, err := a.D.Store.SetUnit(r.Context(), r.PathValue("id"), in.Status, in.Task, in.Step)
	if err != nil {
		fail(w, err)
		return
	}
	a.D.PublishUnit(r.Context(), u)
	httpx.JSON(w, 200, u)
}

func (a *API) shelters(w http.ResponseWriter, r *http.Request) {
	xs, err := a.D.Store.Shelters(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	httpx.Items(w, xs)
}

func (a *API) patchShelter(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Occ  *int  `json:"occ"`
		Open *bool `json:"open"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	x, err := a.D.Store.PatchShelter(r.Context(), r.PathValue("id"), in.Occ, in.Open)
	if err != nil {
		fail(w, err)
		return
	}
	a.D.PublishShelter(r.Context(), x)
	httpx.JSON(w, 200, x)
}

// pharmacies lists shops with stock of one medicine. With lat/lng it ranks like rankPharmacies():
// shops that have it and are reachable first, then by drive time.
func (a *API) pharmacies(w http.ResponseWriter, r *http.Request) {
	ps, err := a.D.Store.Pharmacies(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	med := r.URL.Query().Get("med")
	type row struct {
		P         domain.Pharmacy `json:"p"`
		Qty       int             `json:"qty"`
		Min       *float64        `json:"min"`
		Reachable bool            `json:"reachable"`
	}
	rows := make([]row, len(ps))
	for i, p := range ps {
		rows[i] = row{P: p, Qty: p.Stock[med], Reachable: true}
	}
	lat, e1 := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lng, e2 := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
	if e1 == nil && e2 == nil {
		targets := make([]domain.Target, len(ps))
		for i, p := range ps {
			targets[i] = domain.Target{ID: p.ID, Lat: p.Lat, Lng: p.Lng}
		}
		times, err := a.D.Graph.Times(r.Context(), domain.Point{Lat: lat, Lng: lng}, targets, true)
		for i, p := range ps {
			if err == nil {
				rows[i].Min, rows[i].Reachable = times[p.ID], times[p.ID] != nil
			} else {
				v := domain.EstimateMinutes(domain.Hav(lat, lng, p.Lat, p.Lng))
				rows[i].Min = &v
			}
		}
		sort.SliceStable(rows, func(i, j int) bool {
			ai, aj := rows[i].Qty > 0 && rows[i].Reachable, rows[j].Qty > 0 && rows[j].Reachable
			if ai != aj {
				return ai
			}
			if rows[i].Min == nil || rows[j].Min == nil {
				return rows[i].Min != nil
			}
			return *rows[i].Min < *rows[j].Min
		})
	}
	httpx.Items(w, rows)
}

func (a *API) patchStock(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Med string `json:"med"`
		Qty int    `json:"qty"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	if in.Med == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "med is required")
		return
	}
	if err := a.D.Store.SetStock(r.Context(), r.PathValue("id"), in.Med, in.Qty); err != nil {
		fail(w, err)
		return
	}
	a.D.PublishStock(r.Context(), r.PathValue("id"), in.Med, in.Qty)
	httpx.JSON(w, 200, map[string]any{"id": r.PathValue("id"), "med": in.Med, "qty": in.Qty})
}

func (a *API) approve(w http.ResponseWriter, r *http.Request) {
	var in dispatch.Approve
	if !httpx.Decode(w, r, &in) {
		return
	}
	if in.UnitID == "" && in.HospitalID == "" && in.ShelterID == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "unitId, hospitalId or shelterId is required")
		return
	}
	if err := a.D.Approve(r.Context(), in, true); err != nil {
		fail(w, err)
		return
	}
	a.D.PublishAssignment(r.Context(), in)
	httpx.JSON(w, 200, map[string]any{"status": "approved", "unit": in.UnitID, "hospital": in.HospitalID})
}

func (a *API) assignments(w http.ResponseWriter, r *http.Request) {
	as, err := a.D.Store.Assignments(r.Context(), 100)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.Items(w, as)
}

// stub serves a tier-3 module's stub envelope (Abhayasūchī, Sambharaṇa), labelled as a stub.
func (a *API) stub(key string) http.HandlerFunc {
	return auth.RequireOrKey(a.Secret, a.Key, auth.Controller, auth.Responder)(func(w http.ResponseWriter, _ *http.Request) {
		s, ok := a.Stubs[key]
		if !ok {
			httpx.Error(w, http.StatusNotFound, "not_found", "no stub for this module")
			return
		}
		httpx.JSON(w, 200, s)
	})
}
