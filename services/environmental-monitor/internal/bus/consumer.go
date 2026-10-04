// Package bus turns environmental telemetry (telemetry.raw) into stored readings and rainfall.observed,
// river.level and flood.stage events.
package bus

import (
	"context"
	"fmt"
	"math"
	"sync"

	"nexus/shared/envelope"

	"github.com/nexus/environmental-monitor/internal/domain"
	api "github.com/nexus/environmental-monitor/internal/http"
	"github.com/nexus/environmental-monitor/internal/store"
)

var Keys = []string{"telemetry.raw", "scenario.loaded"}

type Consumer struct {
	Store *store.Store
	API   *api.API
	Pub   api.Publisher

	mu        sync.Mutex
	lastStage float64
}

type reading struct {
	AssetID string  `json:"asset_id"`
	Type    string  `json:"type"`
	Value   float64 `json:"value"`
	Source  string  `json:"source"`
}

func (c *Consumer) Handle(ctx context.Context, env envelope.Envelope) error {
	if env.Key() == "scenario.loaded" { // reload: the flood is back to 0
		c.mu.Lock()
		c.lastStage = 0
		c.mu.Unlock()
		return c.Store.SetFloodStage(ctx, 0, "reset")
	}
	var r reading
	if env.Decode(&r) != nil || r.AssetID == "" {
		return nil
	}
	src := r.Source
	if src == "" {
		src = env.Source
	}
	switch r.Type {
	case "rainfall_mm":
		if err := c.Store.AddRain(ctx, r.AssetID, r.Value, src); err != nil {
			return err
		}
		e := envelope.New("rainfall", "observed", src, "environmental-monitor").WithStatus(fmt.Sprintf("%.1f mm", r.Value)).
			WithPayload(map[string]any{"station": r.AssetID, "mm": r.Value, "source": src})
		return c.Pub.Publish(ctx, e)
	case "river_level_m":
		status, danger := domain.RiverStatus(r.AssetID, r.Value)
		if err := c.Store.AddRiver(ctx, r.AssetID, r.Value, danger, status, src); err != nil {
			return err
		}
		e := envelope.New("river", "level", src, "environmental-monitor").WithStatus(status).
			WithPayload(map[string]any{"river": r.AssetID, "level": r.Value, "danger": danger, "status": status, "source": src})
		return c.Pub.Publish(ctx, e)
	case "flood_stage":
		stage := domain.ClampStage(r.Value)
		c.mu.Lock()
		same := math.Abs(stage-c.lastStage) < 1
		if !same {
			c.lastStage = stage
		}
		c.mu.Unlock()
		if same {
			return nil
		}
		if err := c.Store.SetFloodStage(ctx, stage, src); err != nil {
			return err
		}
		c.API.PublishStage(ctx, stage, src)
	}
	return nil
}
