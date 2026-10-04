// Package bus wires graph-engine to the Sūtra bus: it keeps the live road picture and the
// utility topology in step with road_segment.*, flood.*, scenario.*, asset.* and telemetry.raw.
package bus

import (
	"context"
	"log/slog"

	"nexus/shared/envelope"
	"nexus/shared/seed"

	"github.com/nexus/graph-engine/internal/domain"
	api "github.com/nexus/graph-engine/internal/http"
)

var Keys = []string{"road_segment.*", "flood.*", "asset.*", "scenario.*", "telemetry.raw"}

type Consumer struct {
	Svc       *api.Service
	Scenarios map[string]seed.Scenario
	Log       *slog.Logger
}

type passability struct {
	Kind   string  `json:"kind"` // closure | break
	ID     string  `json:"id"`
	Active bool    `json:"active"`
	Op     string  `json:"op"` // add | remove | clear
	Lat    float64 `json:"lat"`
	Lng    float64 `json:"lng"`
	Name   string  `json:"name"`
}

func (c *Consumer) Handle(ctx context.Context, env envelope.Envelope) error {
	st := c.Svc.Planner.State
	switch env.Key() {
	case "road_segment.passability":
		var p passability
		if err := env.Decode(&p); err != nil {
			return nil // not for us: no machine-readable payload
		}
		switch p.Kind {
		case "closure":
			st.SetClosure(p.ID, p.Active)
		case "break":
			switch p.Op {
			case "add":
				st.AddBreak(domain.Point{Lat: p.Lat, Lng: p.Lng, Name: p.Name})
			case "remove":
				st.RemoveBreak(p.Lat, p.Lng)
			case "clear":
				st.ClearBreaks()
			}
		}
	case "flood.stage":
		var p struct {
			Stage float64 `json:"stage"`
		}
		if env.Decode(&p) == nil {
			st.SetFlood(p.Stage)
		}
	case "scenario.loaded":
		var p struct {
			ID string `json:"id"`
		}
		if env.Decode(&p) == nil {
			if sc, ok := c.Scenarios[p.ID]; ok {
				st.Reset(sc.Closures)
			}
		}
		// "Reset everything" in the console reloads the scenario: heal the utility graph as well.
		for _, a := range c.Svc.Topo.Reset() {
			c.Svc.Persist([]domain.Asset{a})
		}
	case "telemetry.raw":
		var p struct {
			AssetID string  `json:"asset_id"`
			Type    string  `json:"type"`
			Value   float64 `json:"value"`
		}
		if env.Decode(&p) == nil && p.AssetID != "" {
			c.Svc.Ingest(ctx, p.AssetID, p.Value)
		}
	case "asset.fail":
		var p struct {
			ID string `json:"id"`
		}
		if env.Decode(&p) == nil && p.ID != "" {
			c.Svc.Trip(ctx, p.ID, "bus request")
		}
	}
	return nil
}
