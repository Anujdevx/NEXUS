// Package bus applies the dispatch result to the incident: assignment.dispatched sets the unit and logs it.
package bus

import (
	"context"
	"fmt"

	"nexus/shared/envelope"

	"github.com/nexus/citizen-reporting/internal/store"
)

var Keys = []string{"assignment.dispatched", "incident.*"}

type Consumer struct{ Store *store.Store }

func (c *Consumer) Handle(ctx context.Context, env envelope.Envelope) error {
	switch env.Key() {
	case "assignment.dispatched":
		var p struct {
			IncidentID string `json:"incidentId"`
			Unit       string `json:"unit"`
			Hospital   struct {
				N string `json:"n"`
			} `json:"hospital"`
		}
		if env.Decode(&p) != nil || p.IncidentID == "" {
			return nil
		}
		status, text := "Open", "No ambulance free; queued"
		var unit *string
		if p.Unit != "" {
			status, unit = "Unit assigned", &p.Unit
			text = fmt.Sprintf("%s assigned, going to %s", p.Unit, p.Hospital.N)
		}
		ok, err := c.Store.SetStatus(ctx, p.IncidentID, status, unit)
		if err != nil || !ok {
			return err
		}
		_ = c.Store.SetSOSStatus(ctx, p.IncidentID, "dispatched")
		return c.Store.AddLog(ctx, p.IncidentID, text)
	case "incident.acknowledged", "incident.closed":
		// the console and other services publish these; persist when the incident is ours
		var p struct {
			ID string `json:"id"`
		}
		if env.Decode(&p) != nil || p.ID == "" {
			return nil
		}
		st := "Acknowledged"
		if env.Type == "closed" {
			st = "Closed"
		}
		_, err := c.Store.SetStatus(ctx, p.ID, st, nil)
		return err
	}
	return nil
}
