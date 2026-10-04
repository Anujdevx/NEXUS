// Package http holds predictive-analytics' REST handlers and its inference state.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"

	"nexus/shared/auth"
	"nexus/shared/envelope"
	"nexus/shared/httpx"

	"github.com/nexus/predictive-analytics/internal/domain"
)

type Publisher interface {
	Publish(ctx context.Context, env envelope.Envelope) error
}

type API struct {
	Segs       []domain.Seg
	Pub        Publisher
	Secret     string
	Key        string
	GraphURL   string
	TrafficURL string
	HC         *http.Client
	Cache      *Cache

	mu       sync.Mutex
	stations map[string]float64 // latest observed rainfall per station; inference uses the maximum
	mm       float64
	source   string
	inferred map[string]bool
}

func New(segs []domain.Seg, pub Publisher, secret, key, graphURL, trafficURL string, cache *Cache) *API {
	return &API{Segs: segs, Pub: pub, Secret: secret, Key: key, GraphURL: graphURL, TrafficURL: trafficURL, HC: &http.Client{Timeout: 60 * time.Second}, Cache: cache, inferred: map[string]bool{}, stations: map[string]float64{}, source: "none"}
}

func (a *API) Register(app *httpx.App) {
	anyRole := auth.RequireOrKey(a.Secret, a.Key, auth.Controller, auth.Responder) // operational data: not for citizens
	ctl := auth.Require(a.Secret, auth.Controller)
	app.Handle("POST /api/v1/hazard/rain-whatif", anyRole(a.whatIf))
	app.Handle("GET /api/v1/hazard/state", anyRole(a.state))
	app.Handle("POST /api/v1/analytics/evaluation", ctl(a.evaluation))
}

type whatIfIn struct {
	MM      float64 `json:"mm"`
	Publish bool    `json:"publish"`
}

// whatIf infers passability at a rainfall total. With publish:true (controllers) it also moves the shared picture.
func (a *API) whatIf(w http.ResponseWriter, r *http.Request) {
	var in whatIfIn
	if !httpx.Decode(w, r, &in) {
		return
	}
	if in.MM < 0 || in.MM > 1000 {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "mm must be between 0 and 1000")
		return
	}
	closures := domain.InferFromRain(in.MM, a.Segs)
	if in.Publish {
		if c := auth.FromContext(r.Context()); c != nil && c.Role == auth.Controller {
			a.Apply(r.Context(), in.MM, "what-if (api)")
		}
	}
	httpx.JSON(w, 200, map[string]any{
		"mm": in.MM, "segments": domain.Segments(in.MM, a.Segs), "closures": closures, "risk": domain.Risk(in.MM),
		"note": "Inferred, not observed. Thresholds are illustrative, not published Garhwal values.",
	})
}

func (a *API) state(w http.ResponseWriter, _ *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ids := make([]string, 0, len(a.inferred))
	for id := range a.inferred {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	httpx.JSON(w, 200, map[string]any{"mm": a.mm, "source": a.source, "inferred": ids, "risk": domain.Risk(a.mm)})
}

// Reset forgets observed rainfall and the closures the model owns (scenario reload).
func (a *API) Reset() {
	a.mu.Lock()
	a.stations, a.inferred, a.mm, a.source = map[string]float64{}, map[string]bool{}, 0, "none"
	a.mu.Unlock()
}

// Observe records one station's rainfall and applies the maximum over all stations.
func (a *API) Observe(ctx context.Context, station string, mm float64) {
	a.mu.Lock()
	a.stations[station] = mm
	max := 0.0
	for _, v := range a.stations {
		if v > max {
			max = v
		}
	}
	a.mu.Unlock()
	a.Apply(ctx, max, "observed")
}

// activeClosures asks traffic-control which closures are already in force (scenario or manual), so the
// model only flips closures it is responsible for and never reopens a documented one.
func (a *API) activeClosures(ctx context.Context) map[string]bool {
	out := map[string]bool{}
	if a.TrafficURL == "" {
		return out
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.TrafficURL+"/api/v1/roads/state", nil)
	if err != nil {
		return out
	}
	req.Header.Set("X-Service-Key", a.Key)
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return out
	}
	defer resp.Body.Close()
	var st struct {
		Closures []string `json:"closures"`
	}
	if json.NewDecoder(resp.Body).Decode(&st) == nil {
		for _, id := range st.Closures {
			out[id] = true
		}
	}
	return out
}

// Apply is for observed rainfall: it sets the working rainfall, infers passability and publishes what
// changed: one road_segment.passability per closure the model flipped (with a confidence) and one hazard.risk.
func (a *API) Apply(ctx context.Context, mm float64, source string) {
	want := map[string]bool{}
	for _, id := range domain.InferFromRain(mm, a.Segs) {
		want[id] = true
	}
	conf := map[string]domain.Segment{}
	for _, s := range domain.Segments(mm, a.Segs) {
		conf[s.Edge] = s
	}
	active := a.activeClosures(ctx)
	a.mu.Lock()
	owned := a.inferred
	a.mm, a.source = mm, source
	next := map[string]bool{}
	var on, off []string
	for id := range want {
		if owned[id] {
			next[id] = true
		} else if !active[id] {
			next[id] = true
			on = append(on, id)
		}
	}
	for id := range owned {
		if !want[id] {
			off = append(off, id)
		}
	}
	a.inferred = next
	a.mu.Unlock()
	a.Cache.Set(ctx, "hazard:mm", mm)

	flip := func(id string, isOn bool) {
		s := conf[id]
		st := "open"
		if isOn {
			st = s.Kind
		}
		e := envelope.New("road_segment", "passability", "purvasuchana", "predictive-analytics").WithStatus(st).WithConfidence(s.Confidence).
			WithPayload(map[string]any{"kind": "closure", "id": id, "active": isOn, "inferred": true, "rain_mm": mm, "basis": source})
		_ = a.Pub.Publish(ctx, e)
	}
	sort.Strings(on)
	sort.Strings(off)
	for _, id := range on {
		flip(id, true)
	}
	for _, id := range off {
		flip(id, false)
	}
	a.Risk(ctx, mm, source)
}

// Risk publishes hazard.risk for a rainfall total without touching road state (used for the console's what-if,
// whose closure changes the console mirrors itself).
func (a *API) Risk(ctx context.Context, mm float64, source string) {
	a.mu.Lock()
	a.mm, a.source = mm, source
	a.mu.Unlock()
	ids := domain.InferFromRain(mm, a.Segs)
	_ = a.Pub.Publish(ctx, envelope.New("hazard", "risk", "purvasuchana", "predictive-analytics").WithStatus(domain.Risk(mm)).WithConfidence(0.5).
		WithPayload(map[string]any{"mm": mm, "level": domain.Risk(mm), "closures": ids, "basis": source}))
}

// evaluation runs the connectivity sweep. The road graph lives in graph-engine, so the sweep runs
// there and this service fronts it (and caches the result in Redis).
func (a *API) evaluation(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if !httpx.Decode(w, r, &raw) {
		return
	}
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	key := "eval:" + string(raw)
	if hit, ok := a.Cache.GetString(r.Context(), key); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "hit")
		_, _ = w.Write([]byte(hit))
		return
	}
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, a.GraphURL+"/api/v1/routes/evaluate", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Key", a.Key)
	resp, err := a.HC.Do(req)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "graph_unavailable", "graph-engine is not reachable: "+err.Error())
		return
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	if resp.StatusCode == 200 {
		a.Cache.SetString(r.Context(), key, buf.String(), 10*time.Minute)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(buf.Bytes())
}
