// Package dispatch is the orchestrator: it turns an SOS (or an approval) into an assignment,
// updates hospital, unit and shelter state, and publishes the resulting events.
package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"

	"nexus/shared/envelope"

	"github.com/nexus/emergency-dispatch/internal/domain"
	"github.com/nexus/emergency-dispatch/internal/store"
)

type Publisher interface {
	Publish(ctx context.Context, env envelope.Envelope) error
}

type Dispatcher struct {
	Store  *store.Store
	Graph  *domain.Graph
	Pub    Publisher
	Caps   map[string][]string
	Origin string
	Log    *slog.Logger
}

// ---- allocation ----

// Rank ranks hospitals for a patient at a point. The second result says how drive times were obtained.
func (d *Dispatcher) Rank(ctx context.Context, at domain.Point, need string) ([]domain.RankRow, string, error) {
	hs, err := d.Store.Hospitals(ctx)
	if err != nil {
		return nil, "", err
	}
	targets := make([]domain.Target, len(hs))
	for i, h := range hs {
		targets[i] = domain.Target{ID: h.ID, Lat: h.Lat, Lng: h.Lng}
	}
	times, err := d.Graph.Times(ctx, at, targets, true)
	estimator := "road"
	if err != nil {
		d.Log.Warn("graph-engine unavailable, estimating drive times by straight line", "err", err)
		estimator = "haversine"
		times = map[string]*float64{}
		for _, h := range hs {
			v := domain.EstimateMinutes(domain.Hav(at.Lat, at.Lng, h.Lat, h.Lng))
			times[h.ID] = &v
		}
	}
	return domain.Rank(hs, need, times, d.Caps), estimator, nil
}

// pickAmbulance orders available ambulances by drive time to the point and claims the first one that is still free.
func (d *Dispatcher) pickAmbulance(ctx context.Context, at domain.Point, task string) (*domain.Unit, error) {
	units, err := d.Store.Units(ctx)
	if err != nil {
		return nil, err
	}
	var cand []domain.Unit
	for _, u := range units {
		if u.Status == "Available" && u.Type == "Ambulance" {
			cand = append(cand, u)
		}
	}
	if len(cand) == 0 {
		return nil, nil
	}
	targets := make([]domain.Target, len(cand))
	for i, u := range cand {
		targets[i] = domain.Target{ID: u.ID, Lat: u.Lat, Lng: u.Lng}
	}
	times, err := d.Graph.Times(ctx, at, targets, false)
	dist := func(u domain.Unit) float64 {
		if err == nil {
			if t := times[u.ID]; t != nil {
				return *t
			}
			return math.Inf(1)
		}
		return domain.Hav(at.Lat, at.Lng, u.Lat, u.Lng)
	}
	sort.SliceStable(cand, func(i, j int) bool { return dist(cand[i]) < dist(cand[j]) })
	for _, u := range cand {
		ok, err := d.Store.ClaimUnit(ctx, u.ID, task)
		if err != nil {
			return nil, err
		}
		if ok {
			u.Status, u.Task = "On mission", task
			return &u, nil
		}
	}
	return nil, nil
}

// ---- SOS ----

type SOS struct {
	ID        string  `json:"id"`
	Place     string  `json:"place"`
	PlaceName string  `json:"placeName"`
	Hazard    string  `json:"hazard"`
	People    int     `json:"people"`
	Injured   string  `json:"injured"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Node      string  `json:"node"`
	Via       string  `json:"via"`
	Client    string  `json:"client"`
}

type allocRow struct {
	ID        string   `json:"id"`
	N         string   `json:"n"`
	Lvl       string   `json:"lvl"`
	Min       *float64 `json:"min"`
	Reachable bool     `json:"reachable"`
	Capable   bool     `json:"capable"`
	Free      int      `json:"free"`
	Inbound   int      `json:"inbound"`
	Cost      float64  `json:"cost"`
}

func compact(rows []domain.RankRow, n int) []allocRow {
	if len(rows) < n {
		n = len(rows)
	}
	out := make([]allocRow, n)
	for i := 0; i < n; i++ {
		r := rows[i]
		out[i] = allocRow{r.H.ID, r.H.N, r.H.Lvl, r.Min, r.Reachable, r.Capable, r.Free, r.Inbound, r.Cost}
	}
	return out
}

// HandleSOS allocates a hospital and an ambulance for one SOS and publishes the assignment.
func (d *Dispatcher) HandleSOS(ctx context.Context, env envelope.Envelope) error {
	var s SOS
	if err := env.Decode(&s); err != nil || s.ID == "" {
		return nil // not a dispatchable SOS
	}
	need := "General"
	if s.Injured == "Yes" {
		need = "Trauma"
	}
	at := domain.Point{Lat: s.Lat, Lng: s.Lng, Name: s.PlaceName}
	rows, estimator, err := d.Rank(ctx, at, need)
	if err != nil || len(rows) == 0 {
		return fmt.Errorf("rank: %v", err)
	}
	top := rows[0]
	hs := make([]domain.Hospital, len(rows))
	for i, r := range rows {
		hs[i] = r.H
	}
	base := domain.NearestHospital(hs, s.Lat, s.Lng)
	task := fmt.Sprintf("SOS: %s, %d people", s.Hazard, s.People)
	unit, err := d.pickAmbulance(ctx, at, task)
	if err != nil {
		return err
	}
	h, err := d.Store.BumpInbound(ctx, top.H.ID, 1)
	if err != nil {
		return err
	}
	var route *domain.RouteSummary
	if rs, err := d.Graph.Plan(ctx, at, domain.Point{Lat: top.H.Lat, Lng: top.H.Lng}); err == nil {
		route = rs
	}
	unitID, status := "", "queued"
	if unit != nil {
		unitID, status = unit.ID, unit.ID
	}
	payload := map[string]any{
		"kind": "sos", "incidentId": s.ID, "sosId": s.ID, "placeName": s.PlaceName, "unit": unitID, "need": need, "via": s.Via, "client": s.Client,
		"hospital": compact(rows, 1)[0], "baseline": map[string]string{"id": base.ID, "n": base.N}, "alloc": compact(rows, 5),
		"estimator": estimator, "route": route,
	}
	routeJSON, _ := json.Marshal(route)
	_ = d.Store.InsertAssignment(ctx, store.Assignment{ID: "asg-" + s.ID, IncidentID: s.ID, UnitID: unitID, HospitalID: top.H.ID, Route: routeJSON})

	_ = d.Pub.Publish(ctx, envelope.New("assignment", "dispatched", "dispatch", d.Origin).WithGeo(s.Lat, s.Lng).WithStatus(status).WithPayload(payload))
	d.publishHospital(ctx, h, "inbound")
	if unit != nil {
		d.publishUnit(ctx, *unit)
	}
	d.Log.Info("sos dispatched", "sos", s.ID, "hospital", top.H.ID, "unit", unitID, "estimator", estimator)
	return nil
}

// ---- approval (Routes screen, incident dispatch, REST) ----

type Approve struct {
	IncidentID string          `json:"incidentId"`
	UnitID     string          `json:"unitId"`
	HospitalID string          `json:"hospitalId"`
	ShelterID  string          `json:"shelterId"`
	People     int             `json:"people"`
	From       string          `json:"from"`
	To         string          `json:"to"`
	Route      json.RawMessage `json:"route"`
}

// Approve records a controller's approved assignment. It sets rather than claims, so it is idempotent
// when the console has already applied the same change locally.
func (d *Dispatcher) Approve(ctx context.Context, a Approve, publish bool) error {
	if a.UnitID != "" {
		task := a.To
		if a.From != "" {
			task = a.From + " to " + a.To
		}
		u, err := d.Store.SetUnit(ctx, a.UnitID, "On mission", &task, nil)
		if err != nil {
			return err
		}
		if publish {
			d.publishUnit(ctx, u)
		}
	}
	if a.HospitalID != "" {
		h, err := d.Store.BumpInbound(ctx, a.HospitalID, 1)
		if err != nil {
			return err
		}
		if publish {
			d.publishHospital(ctx, h, "inbound")
		}
	}
	if a.ShelterID != "" {
		people := a.People
		if people == 0 {
			people = 10
		}
		if x, err := d.Store.AddOccupancy(ctx, a.ShelterID, people); err == nil && publish {
			d.publishShelter(ctx, x)
		}
	}
	id := fmt.Sprintf("asg-%s-%s", a.UnitID, envelope.New("", "", "", "").ID[:8])
	return d.Store.InsertAssignment(ctx, store.Assignment{ID: id, IncidentID: a.IncidentID, UnitID: a.UnitID, HospitalID: a.HospitalID, Route: a.Route})
}

// ---- publishing state changes ----

func (d *Dispatcher) publishHospital(ctx context.Context, h domain.Hospital, why string) {
	st := "accepting"
	if h.Divert {
		st = "diverting"
	}
	e := envelope.New("hospital", "capacity", "arogya", d.Origin).WithGeo(h.Lat, h.Lng).WithStatus(st).
		WithCapacity(map[string]any{"free": h.Free, "inbound": h.Inbound, "cap": h.Cap}).
		WithPayload(map[string]any{"id": h.ID, "free": h.Free, "inbound": h.Inbound, "divert": h.Divert, "cap": h.Cap, "why": why})
	_ = d.Pub.Publish(ctx, e)
}

func (d *Dispatcher) publishUnit(ctx context.Context, u domain.Unit) {
	e := envelope.New("unit", "status", "rakshaka", d.Origin).WithGeo(u.Lat, u.Lng).WithStatus(u.Status).
		WithPayload(map[string]any{"id": u.ID, "unit": u.ID, "status": u.Status, "task": u.Task, "step": u.Step})
	_ = d.Pub.Publish(ctx, e)
}

func (d *Dispatcher) publishShelter(ctx context.Context, x domain.Shelter) {
	st := "open"
	if x.Occ >= x.Cap {
		st = "full"
	} else if !x.Open {
		st = "closed"
	}
	e := envelope.New("shelter", "occupancy", "ashraya", d.Origin).WithGeo(x.Lat, x.Lng).WithStatus(st).
		WithCapacity(map[string]any{"cap": x.Cap, "occ": x.Occ}).WithPayload(map[string]any{"id": x.ID, "occ": x.Occ, "open": x.Open, "cap": x.Cap})
	_ = d.Pub.Publish(ctx, e)
}

func (d *Dispatcher) PublishHospital(ctx context.Context, h domain.Hospital, why string) {
	d.publishHospital(ctx, h, why)
}
func (d *Dispatcher) PublishUnit(ctx context.Context, u domain.Unit)       { d.publishUnit(ctx, u) }
func (d *Dispatcher) PublishShelter(ctx context.Context, x domain.Shelter) { d.publishShelter(ctx, x) }

func (d *Dispatcher) PublishStock(ctx context.Context, pid, med string, qty int) {
	e := envelope.New("pharmacy", "stock", "arogya", d.Origin).WithStatus(med).WithCapacity(map[string]any{"qty": qty}).
		WithPayload(map[string]any{"id": pid, "med": med, "qty": qty})
	_ = d.Pub.Publish(ctx, e)
}

// PublishAssignment announces an approved assignment (REST path; the console publishes its own on the bus path).
func (d *Dispatcher) PublishAssignment(ctx context.Context, a Approve) {
	st := a.UnitID
	e := envelope.New("assignment", "dispatched", "dispatch", d.Origin).WithStatus(st).
		WithPayload(map[string]any{"kind": "approved", "incidentId": a.IncidentID, "unit": a.UnitID, "hospital": a.HospitalID, "shelter": a.ShelterID, "from": a.From, "to": a.To})
	_ = d.Pub.Publish(ctx, e)
}
