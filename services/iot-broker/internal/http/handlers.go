// Package http holds iot-broker's REST and WebSocket endpoints: event mirror and audit trail,
// telemetry ingest, the live stream.
package http

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/coder/websocket"

	"nexus/shared/auth"
	"nexus/shared/bus"
	"nexus/shared/envelope"
	"nexus/shared/httpx"

	"github.com/nexus/iot-broker/internal/hub"
	"github.com/nexus/iot-broker/internal/store"
)

type API struct {
	Bus    *bus.Bus
	Store  *store.Store
	Hub    *hub.Hub
	Secret string
	Key    string

	mu     sync.Mutex
	latest map[string]map[string]any // asset id -> latest telemetry reading
}

func New(b *bus.Bus, s *store.Store, h *hub.Hub, secret, key string) *API {
	return &API{Bus: b, Store: s, Hub: h, Secret: secret, Key: key, latest: map[string]map[string]any{}}
}

func (a *API) Register(app *httpx.App) {
	anyRole := auth.Require(a.Secret)
	app.Handle("POST /api/v1/events", anyRole(a.postEvent))
	app.Handle("GET /api/v1/events", anyRole(a.listEvents))
	app.Handle("POST /api/v1/telemetry/ingest", auth.ServiceKey(a.Key)(a.ingest))
	app.Handle("GET /api/v1/telemetry/latest", anyRole(a.latestTelemetry))
	app.Handle("GET /ws/v1/live", a.live)
}

// postEvent is the frontend's mirror of publish(): it puts the envelope on the bus. Citizens may only
// publish what a citizen device produces.
func (a *API) postEvent(w http.ResponseWriter, r *http.Request) {
	var env envelope.Envelope
	if !httpx.Decode(w, r, &env) {
		return
	}
	if env.Entity == "" || env.Type == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_envelope", "entity and type are required")
		return
	}
	c := auth.FromContext(r.Context())
	if c.Role == auth.Citizen && env.Entity != "telecom" && env.Entity != "sos" {
		httpx.Error(w, http.StatusForbidden, "forbidden", "citizens may only publish sos and telecom events")
		return
	}
	env.Fill("web")
	if err := a.Bus.Publish(r.Context(), env); err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "bus_unavailable", err.Error())
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]string{"id": env.ID, "key": env.Key()})
}

func (a *API) listEvents(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	if c.Role == auth.Citizen {
		httpx.Error(w, http.StatusForbidden, "forbidden", "the audit trail is for controllers and responders")
		return
	}
	limit := 200
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 2000 {
		limit = n
	}
	rows, err := a.Store.Recent(r.Context(), limit, r.URL.Query().Get("key"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"items": rows, "count": len(rows)})
}

type reading struct {
	AssetID   string  `json:"asset_id"`
	Type      string  `json:"type"`
	Value     float64 `json:"value"`
	Timestamp string  `json:"timestamp"`
	Source    string  `json:"source"`
}

// ingest takes one reading, or {"items": [...]}, from a sensor or the simulator and publishes telemetry.raw.
func (a *API) ingest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		reading
		Items []reading `json:"items"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	list := in.Items
	if in.AssetID != "" {
		list = append(list, in.reading)
	}
	if len(list) == 0 {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "asset_id, type, value and timestamp are required")
		return
	}
	for _, x := range list {
		if x.AssetID == "" || x.Type == "" {
			httpx.Error(w, http.StatusBadRequest, "bad_request", "asset_id and type are required")
			return
		}
		if x.Timestamp == "" {
			x.Timestamp = time.Now().UTC().Format(time.RFC3339)
		}
		src := x.Source
		if src == "" {
			src = "sensor"
		}
		env := envelope.New("telemetry", "raw", src, "iot-broker").WithPayload(x)
		if err := a.Bus.Publish(r.Context(), env); err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "bus_unavailable", err.Error())
			return
		}
		a.mu.Lock()
		a.latest[x.AssetID] = map[string]any{"asset_id": x.AssetID, "type": x.Type, "value": x.Value, "timestamp": x.Timestamp, "source": src}
		a.mu.Unlock()
	}
	httpx.JSON(w, http.StatusAccepted, map[string]int{"accepted": len(list)})
}

func (a *API) latestTelemetry(w http.ResponseWriter, _ *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	items := make([]map[string]any, 0, len(a.latest))
	for _, v := range a.latest {
		items = append(items, v)
	}
	httpx.Items(w, items)
}

// live upgrades to a WebSocket and streams envelopes filtered for the caller's role.
func (a *API) live(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.Parse(a.Secret, auth.Bearer(r))
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "a valid ?token= is required")
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true}) // origin is the dev proxy / gateway
	if err != nil {
		return
	}
	id := r.URL.Query().Get("client")
	if id == "" {
		id = "anon-" + strconv.FormatInt(time.Now().UnixNano()%1_000_000, 36)
	}
	c := &hub.Client{ID: id, Role: claims.Role, Unit: r.URL.Query().Get("unit")}
	a.Hub.Serve(r.Context(), conn, c)
}
