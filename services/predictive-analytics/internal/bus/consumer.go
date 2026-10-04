// Package bus feeds the hazard model: observed rainfall (and the console's what-if) drive inference.
package bus

import (
	"context"

	"nexus/shared/envelope"

	api "github.com/nexus/predictive-analytics/internal/http"
)

var Keys = []string{"rainfall.*", "river.*", "scenario.loaded"}

type Consumer struct{ API *api.API }

func (c *Consumer) Handle(ctx context.Context, env envelope.Envelope) error {
	switch env.Key() {
	case "scenario.loaded":
		c.API.Reset()
	case "rainfall.observed", "rainfall.what_if":
		var p struct {
			MM      float64 `json:"mm"`
			Station string  `json:"station"`
		}
		if env.Decode(&p) != nil {
			return nil
		}
		if env.Type == "what_if" {
			c.API.Risk(ctx, p.MM, "what-if") // the console mirrors its own closure changes
			return nil
		}
		st := p.Station
		if st == "" {
			st = "unnamed"
		}
		c.API.Observe(ctx, st, p.MM)
	case "river.level":
		var p struct {
			River  string `json:"river"`
			Status string `json:"status"`
		}
		if env.Decode(&p) == nil && p.Status == "danger" {
			// a river at its danger mark is a risk signal in its own right
			_ = c.API.Pub.Publish(ctx, envelope.New("hazard", "risk", "purvasuchana", "predictive-analytics").WithStatus("severe").WithConfidence(0.5).
				WithPayload(map[string]any{"level": "severe", "river": p.River, "basis": "river gauge at danger mark"}))
		}
	}
	return nil
}
