// Package http holds the graph-engine REST handlers: routing (A* on the road graph),
// reach, drive-time matrices, and the Prāṇadhārā topology and cascade.
package http

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"nexus/shared/auth"
	"nexus/shared/envelope"
	"nexus/shared/httpx"

	"github.com/nexus/graph-engine/internal/domain"
)

// Publisher is the slice of the bus handlers need.
type Publisher interface {
	Publish(ctx context.Context, env envelope.Envelope) error
}

type Service struct {
	Net     *domain.Net
	Planner *domain.Planner
	Topo    *domain.Topology
	Nodes   map[string]domain.NodePos
	World   domain.EvalWorld
	Pub     Publisher
	Persist func(assets []domain.Asset) // best-effort write to Neo4j
	Secret  string
	Key     string
	Origin  string
}

func (s *Service) Register(app *httpx.App) {
	any := auth.RequireOrKey(s.Secret, s.Key, auth.Controller, auth.Responder) // operational data: not for citizens
	ctl := auth.Require(s.Secret, auth.Controller)
	app.Handle("GET /api/v1/graph/network", any(s.network))
	app.Handle("GET /api/v1/graph/state", any(s.state))
	app.Handle("GET /api/v1/graph/edges/{id}", any(s.edge))
	app.Handle("POST /api/v1/routes/plan", any(s.plan))
	app.Handle("POST /api/v1/routes/reach", any(s.reach))
	app.Handle("POST /api/v1/routes/times", any(s.times))
	app.Handle("POST /api/v1/routes/evaluate", auth.RequireOrKey(s.Secret, s.Key, auth.Controller)(s.evaluate))
	app.Handle("GET /api/v1/topology/assets", any(s.assets))
	app.Handle("GET /api/v1/topology/blast-radius/{id}", any(s.blast))
	app.Handle("POST /api/v1/topology/assets/{id}/fail", auth.RequireOrKey(s.Secret, s.Key, auth.Controller)(s.fail))
	app.Handle("POST /api/v1/topology/assets/{id}/restore", auth.RequireOrKey(s.Secret, s.Key, auth.Controller)(s.restore))
	app.Handle("POST /api/v1/topology/reset", auth.RequireOrKey(s.Secret, s.Key, auth.Controller)(s.reset))
	app.Handle("POST /api/v1/topology/upload", ctl(s.upload))
}

// ---- locations ----

// Loc accepts "nodeId", {"lat","lng"} or {"nodeId"}.
type Loc struct {
	Lat, Lng float64
	Node     string
	set      bool
}

func (l *Loc) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		l.Node, l.set = s, true
		return nil
	}
	var o struct {
		Lat    *float64 `json:"lat"`
		Lng    *float64 `json:"lng"`
		NodeID string   `json:"nodeId"`
		Node   string   `json:"node"`
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return err
	}
	l.Node = o.NodeID
	if l.Node == "" {
		l.Node = o.Node
	}
	if o.Lat != nil && o.Lng != nil {
		l.Lat, l.Lng = *o.Lat, *o.Lng
	}
	l.set = l.Node != "" || o.Lat != nil
	return nil
}

func (s *Service) resolve(l Loc) (int, bool) {
	if !l.set {
		return 0, false
	}
	if l.Node != "" {
		n, ok := s.Nodes[l.Node]
		if !ok {
			return 0, false
		}
		return s.Net.NearestNode(n.Lat, n.Lng), true
	}
	return s.Net.NearestNode(l.Lat, l.Lng), true
}

// ---- request context overrides ----

type ctxIn struct {
	Breaks   []json.RawMessage `json:"breaks"`
	Closures *[]string         `json:"closures"`
	Flood    *float64          `json:"flood"`
	Conf     *float64          `json:"conf"`
	Hard     bool              `json:"hard"`
}

func (c ctxIn) override() domain.Override {
	o := domain.Override{Closures: c.Closures, Flood: c.Flood, Conf: c.Conf, Hard: c.Hard}
	if c.Breaks != nil {
		pts := []domain.Point{}
		for _, raw := range c.Breaks {
			var id int
			if json.Unmarshal(raw, &id) == nil {
				o.BreakIDs = append(o.BreakIDs, id)
				continue
			}
			var p domain.Point
			if json.Unmarshal(raw, &p) == nil {
				pts = append(pts, p)
			}
		}
		o.Breaks = &pts
	}
	return o
}

// ---- handlers ----

func (s *Service) network(w http.ResponseWriter, _ *http.Request) {
	i := s.Net.Describe()
	httpx.JSON(w, 200, map[string]any{"source": i.Source, "nodes": i.Nodes, "edges": i.Edges, "label": i.Label})
}

func (s *Service) state(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, 200, s.Planner.State.Snapshot())
}

// edge describes one road segment: its name and midpoint (traffic-control resolves breaks reported by edge id).
func (s *Service) edge(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 0 || id >= len(s.Net.Edges) {
		httpx.Error(w, http.StatusNotFound, "not_found", "no such edge")
		return
	}
	e := s.Net.Edges[id]
	lat, lng := s.Net.EdgeMid(e)
	httpx.JSON(w, 200, map[string]any{"id": e.I, "name": e.Name, "class": e.HW, "km": e.Km, "minutes": e.Min, "lat": lat, "lng": lng, "a": e.A, "b": e.B})
}

type planIn struct {
	ctxIn
	From    Loc  `json:"from"`
	To      Loc  `json:"to"`
	Publish bool `json:"publish"`
}

func (s *Service) plan(w http.ResponseWriter, r *http.Request) {
	var in planIn
	if !httpx.Decode(w, r, &in) {
		return
	}
	a, ok1 := s.resolve(in.From)
	b, ok2 := s.resolve(in.To)
	if !ok1 || !ok2 {
		httpx.Error(w, http.StatusBadRequest, "bad_location", "from and to need a nodeId or lat/lng")
		return
	}
	ctx := s.Planner.Context(in.override())
	rt := s.Net.Shortest(a, b, ctx)
	if rt == nil {
		httpx.Error(w, http.StatusNotFound, "no_route", "no road joins these two points")
		return
	}
	body := s.routeBody(rt)
	if in.Publish {
		status := "passable"
		if len(rt.Crosses) > 0 {
			status = "cut_off"
		}
		env := envelope.New("route", "computed", "marga", s.Origin).WithStatus(status).WithConfidence(ctx.Conf).
			WithGeo(s.Net.Lat[a], s.Net.Lng[a]).WithPayload(map[string]any{"km": rt.Km, "minutes": rt.Min, "steps": len(rt.Steps)})
		_ = s.Pub.Publish(r.Context(), env)
	}
	httpx.JSON(w, 200, body)
}

func (s *Service) routeBody(rt *domain.Route) map[string]any {
	lngLat := make([][2]float64, len(rt.Coords))
	for i, c := range rt.Coords {
		lngLat[i] = [2]float64{c[1], c[0]}
	}
	steps := make([]map[string]any, len(rt.Steps))
	slow := false
	names := []string{}
	seen := map[string]bool{}
	for i, st := range rt.Steps {
		steps[i] = map[string]any{"edge": st.Edges[0], "name": st.Name, "km": st.Km, "min": st.Min, "edges": st.Edges, "slow": st.Slow, "cut": st.Cut}
		slow = slow || st.Slow
	}
	for _, e := range rt.Crosses {
		if n := s.Net.Edges[e].Name; !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	return map[string]any{
		"path": rt.Nodes, "nodes": rt.Nodes, "edges": rt.Edges, "coords": lngLat, "latlng": rt.Coords,
		"km": rt.Km, "minutes": rt.Min, "min": rt.Min, "steps": steps, "crosses": rt.Crosses,
		"blocked": names, "slow": slow, "graph": s.Net.Describe(),
	}
}

func (s *Service) reach(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ctxIn
		From       Loc     `json:"from"`
		MaxMinutes float64 `json:"maxMinutes"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	src, ok := s.resolve(in.From)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "bad_location", "from needs a nodeId or lat/lng")
		return
	}
	in.Hard = true
	d := s.Net.DistFrom(src, s.Planner.Context(in.override()))
	max := in.MaxMinutes
	if max <= 0 {
		max = 90
	}
	out := make([]map[string]any, 0, 1024)
	for _, e := range s.Net.Edges {
		if t := math.Max(d[e.A], d[e.B]); t <= max {
			out = append(out, map[string]any{"id": e.I, "minutes": t})
		}
	}
	httpx.JSON(w, 200, map[string]any{"edges": out, "graph": s.Net.Describe()})
}

// times answers "how long from here to each of these places" with one Dijkstra: what hospital ranking needs.
func (s *Service) times(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ctxIn
		From    Loc   `json:"from"`
		Hard    *bool `json:"hard"` // default true; false weighs broken roads by report confidence, like unitDist() does
		Targets []struct {
			ID  string  `json:"id"`
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"targets"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	src, ok := s.resolve(in.From)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "bad_location", "from needs a nodeId or lat/lng")
		return
	}
	o := in.override()
	o.Hard = in.Hard == nil || *in.Hard
	d := s.Net.DistFrom(src, s.Planner.Context(o))
	res := map[string]any{}
	for _, t := range in.Targets {
		if v := d[s.Net.NearestNode(t.Lat, t.Lng)]; !math.IsInf(v, 1) {
			res[t.ID] = v
		} else {
			res[t.ID] = nil
		}
	}
	httpx.JSON(w, 200, map[string]any{"times": res, "graph": s.Net.Describe()})
}

// evaluate runs the connectivity sweep (simulation.js) on the loaded road network under the live road state.
// predictive-analytics exposes it as POST /api/v1/analytics/evaluation.
func (s *Service) evaluate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ctxIn
		Runs    int     `json:"runs"`
		SosLoad int     `json:"sosLoad"`
		Seed    *uint32 `json:"seed"`
		Relay   float64 `json:"relay"`
		Retry   float64 `json:"retry"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	p := domain.DefaultEval()
	if in.Runs > 0 {
		p.Runs = in.Runs
	}
	if in.SosLoad > 0 {
		p.NSos = in.SosLoad
	}
	if in.Seed != nil {
		p.Seed = *in.Seed
	}
	if in.Relay > 0 {
		p.Relay = in.Relay
	}
	if in.Retry > 0 {
		p.Retry = in.Retry
	}
	if p.Runs > 100 || p.NSos > 200 {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "runs must be at most 100 and sosLoad at most 200")
		return
	}
	ctx := s.Planner.Context(in.override())
	start := time.Now()
	rows := domain.Sweep(s.Net, s.World, ctx.Blocked, ctx.Slow, p)
	httpx.JSON(w, 200, map[string]any{"items": rows, "params": p, "graph": s.Net.Describe(), "ms": time.Since(start).Milliseconds(),
		"note": "Simulation results, not field results. SOS load, hospital capacity and connectivity are generated."})
}

// ---- topology and cascade ----

func (s *Service) assets(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, 200, map[string]any{"items": s.Topo.List(), "dependencies": s.Topo.DepList(), "source": "simulated"})
}

func (s *Service) blast(w http.ResponseWriter, r *http.Request) {
	res, ok := s.Topo.Fail(r.PathValue("id"), false)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "not_found", "unknown asset")
		return
	}
	httpx.JSON(w, 200, map[string]any{"root": res.Root, "failed": res.Failed, "degraded": res.Degraded, "waves": res.Waves})
}

// Trip runs the cascade for real: persists, then publishes asset.status per change and one cascade.computed.
func (s *Service) Trip(ctx context.Context, root, reason string) (domain.Cascade, bool) {
	res, ok := s.Topo.Fail(root, true)
	if !ok {
		return res, false
	}
	s.announce(ctx, res, reason)
	return res, true
}

func (s *Service) announce(ctx context.Context, res domain.Cascade, reason string) {
	if s.Persist != nil {
		s.Persist(res.Changed)
	}
	for _, a := range res.Changed {
		env := envelope.New("asset", "status", "simulated", s.Origin).WithStatus(a.Status).WithGeo(a.Lat, a.Lng).WithPayload(a)
		_ = s.Pub.Publish(ctx, env)
		if a.Type == "HOSPITAL" && (a.Status == domain.Failed || a.Status == domain.Degraded) {
			cap := map[string]any{"free": 0}
			hc := envelope.New("hospital", "capacity", "simulated", s.Origin).WithStatus("diverting").WithGeo(a.Lat, a.Lng).WithCapacity(cap).
				WithPayload(map[string]any{"id": a.Ref, "divert": true, "reason": "utility " + a.Status + " (" + reason + ")", "asset": a.ID})
			_ = s.Pub.Publish(ctx, hc)
		}
	}
	pl := map[string]any{"root": res.Root, "waves": res.Waves, "failed": res.Failed, "degraded": res.Degraded, "reason": reason}
	var rootAsset domain.Asset
	if a, ok := s.Topo.Get(res.Root); ok {
		rootAsset = a
		pl["rootName"], pl["rootType"] = a.Name, a.Type
	}
	env := envelope.New("cascade", "computed", "simulated", s.Origin).WithStatus("computed").WithPayload(pl)
	if rootAsset.ID != "" {
		env = env.WithGeo(rootAsset.Lat, rootAsset.Lng)
	}
	_ = s.Pub.Publish(ctx, env)
}

func (s *Service) fail(w http.ResponseWriter, r *http.Request) {
	res, ok := s.Trip(r.Context(), r.PathValue("id"), "manual")
	if !ok {
		httpx.Error(w, http.StatusNotFound, "not_found", "unknown asset")
		return
	}
	httpx.JSON(w, 200, map[string]any{"root": res.Root, "failed": res.Failed, "degraded": res.Degraded, "waves": res.Waves})
}

func (s *Service) restore(w http.ResponseWriter, r *http.Request) {
	a, ok := s.Topo.Restore(r.PathValue("id"))
	if !ok {
		httpx.Error(w, http.StatusNotFound, "not_found", "unknown asset")
		return
	}
	s.persistOne(a)
	_ = s.Pub.Publish(r.Context(), envelope.New("asset", "status", "simulated", s.Origin).WithStatus(a.Status).WithGeo(a.Lat, a.Lng).WithPayload(a))
	if a.Type == "HOSPITAL" {
		_ = s.Pub.Publish(r.Context(), envelope.New("hospital", "capacity", "simulated", s.Origin).WithStatus("accepting").WithGeo(a.Lat, a.Lng).WithPayload(map[string]any{"id": a.Ref, "divert": false, "asset": a.ID}))
	}
	httpx.JSON(w, 200, a)
}

func (s *Service) reset(w http.ResponseWriter, r *http.Request) {
	ch := s.Topo.Reset()
	if s.Persist != nil && len(ch) > 0 {
		s.Persist(ch)
	}
	for _, a := range ch {
		_ = s.Pub.Publish(r.Context(), envelope.New("asset", "status", "simulated", s.Origin).WithStatus(a.Status).WithGeo(a.Lat, a.Lng).WithPayload(a))
		if a.Type == "HOSPITAL" {
			_ = s.Pub.Publish(r.Context(), envelope.New("hospital", "capacity", "simulated", s.Origin).WithStatus("accepting").WithGeo(a.Lat, a.Lng).WithPayload(map[string]any{"id": a.Ref, "divert": false, "asset": a.ID}))
		}
	}
	httpx.JSON(w, 200, map[string]any{"restored": len(ch)})
}

func (s *Service) persistOne(a domain.Asset) {
	if s.Persist != nil {
		s.Persist([]domain.Asset{a})
	}
}

// Ingest handles a telemetry load reading for a utility asset (from telemetry.raw on the bus).
func (s *Service) Ingest(ctx context.Context, assetID string, load float64) {
	trip, known := s.Topo.Telemetry(assetID, load, time.Now())
	if known && trip {
		s.Trip(ctx, assetID, "load above failure threshold beyond its grace period")
	}
}

// upload accepts GeoJSON: Point features become assets; properties.depends_on lists [{id, criticality}].
func (s *Service) upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "send multipart form data with a GeoJSON file")
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "missing form file field \"file\"")
		return
	}
	defer f.Close()
	var fc struct {
		Features []struct {
			Geometry struct {
				Type        string          `json:"type"`
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				ID        string  `json:"id"`
				Name      string  `json:"name"`
				Type      string  `json:"type"`
				Load      float64 `json:"load_percentage"`
				Threshold float64 `json:"failure_threshold"`
				GraceMs   int     `json:"grace_period_ms"`
				DependsOn []struct {
					ID          string  `json:"id"`
					Criticality float64 `json:"criticality"`
				} `json:"depends_on"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.NewDecoder(f).Decode(&fc); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_geojson", err.Error())
		return
	}
	var assets []domain.Asset
	var deps []domain.Dep
	for _, ft := range fc.Features {
		if ft.Geometry.Type != "Point" || ft.Properties.ID == "" {
			continue
		}
		var c []float64 // GeoJSON is [lng, lat]
		if json.Unmarshal(ft.Geometry.Coordinates, &c) != nil || len(c) < 2 {
			continue
		}
		p := ft.Properties
		if p.Threshold == 0 {
			p.Threshold = 100
		}
		if p.GraceMs == 0 {
			p.GraceMs = 5000
		}
		if p.Type == "" {
			p.Type = "POWER"
		}
		a := domain.Asset{ID: p.ID, Name: p.Name, Type: p.Type, Lat: c[1], Lng: c[0], Load: p.Load, BaseLoad: p.Load, Threshold: p.Threshold, GraceMs: p.GraceMs, Source: "uploaded", Region: "dehradun"}
		s.Topo.Put(a)
		assets = append(assets, a)
		for _, d := range p.DependsOn {
			dep := domain.Dep{Dependent: p.ID, Provider: d.ID, Criticality: d.Criticality, GraceMs: p.GraceMs}
			s.Topo.AddDep(dep)
			deps = append(deps, dep)
		}
	}
	sort.Slice(assets, func(i, j int) bool { return assets[i].ID < assets[j].ID })
	httpx.JSON(w, 200, map[string]any{"assets": len(assets), "edges": len(deps)})
}
