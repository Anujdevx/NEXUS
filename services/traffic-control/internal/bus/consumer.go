// Package bus keeps traffic-control's records in step with changes made elsewhere (the console, inference).
package bus

import (
	"context"
	"fmt"

	"nexus/shared/envelope"
	"nexus/shared/seed"

	"github.com/nexus/traffic-control/internal/store"
)

var Keys = []string{"scenario.loaded", "road_segment.*"}

type Consumer struct {
	Store     *store.Store
	Scenarios map[string]seed.Scenario
}

func (c *Consumer) Handle(ctx context.Context, env envelope.Envelope) error {
	switch env.Key() {
	case "scenario.loaded":
		var p struct {
			ID string `json:"id"`
		}
		if env.Decode(&p) == nil {
			if sc, ok := c.Scenarios[p.ID]; ok {
				return c.Store.ResetScenario(ctx, sc.Closures)
			}
		}
	case "road_segment.passability":
		var p struct {
			Kind   string  `json:"kind"`
			ID     string  `json:"id"`
			Active bool    `json:"active"`
			Op     string  `json:"op"`
			Lat    float64 `json:"lat"`
			Lng    float64 `json:"lng"`
			Name   string  `json:"name"`
		}
		if env.Decode(&p) != nil {
			return nil
		}
		switch p.Kind {
		case "closure":
			_, err := c.Store.SetClosure(ctx, p.ID, &p.Active)
			if err == store.ErrNotFound {
				return nil
			}
			return err
		case "break":
			switch p.Op {
			case "add":
				return c.Store.AddBreak(ctx, store.Break{EdgeID: fmt.Sprintf("p:%.5f,%.5f", p.Lat, p.Lng), Reason: "reported", CreatedBy: env.Origin, Lat: p.Lat, Lng: p.Lng, Name: p.Name})
			case "remove":
				return c.Store.RemoveBreakNear(ctx, p.Lat, p.Lng)
			case "clear":
				return c.Store.ClearBreaks(ctx)
			}
		}
	}
	return nil
}
