// Package bus applies envelopes from other producers (the console mirror, graph-engine, citizen-reporting)
// to dispatch's state, so every service and every console sees one picture.
package bus

import (
	"context"
	"log/slog"

	"nexus/shared/envelope"

	"github.com/nexus/emergency-dispatch/internal/dispatch"
)

var Keys = []string{"sos.request", "incident.*", "hospital.capacity", "unit.*", "shelter.*", "pharmacy.stock", "assignment.dispatched", "scenario.loaded"}

type Consumer struct {
	D    *dispatch.Dispatcher
	Seed func(ctx context.Context) error // reload seeds (scenario reset)
	Log  *slog.Logger
}

func (c *Consumer) Handle(ctx context.Context, env envelope.Envelope) error {
	st := c.D.Store
	switch env.Key() {
	case "sos.request":
		return c.D.HandleSOS(ctx, env)
	case "assignment.dispatched":
		var p struct {
			Kind       string `json:"kind"`
			Unit       string `json:"unit"`
			Hospital   string `json:"hospital"`
			Shelter    string `json:"shelter"`
			People     int    `json:"people"`
			IncidentID string `json:"incidentId"`
			From       string `json:"from"`
			To         string `json:"to"`
		}
		if env.Decode(&p) == nil && p.Kind == "approval" {
			return c.D.Approve(ctx, dispatch.Approve{IncidentID: p.IncidentID, UnitID: p.Unit, HospitalID: p.Hospital, ShelterID: p.Shelter, People: p.People, From: p.From, To: p.To}, false)
		}
	case "hospital.capacity":
		var p struct {
			ID      string `json:"id"`
			Free    *int   `json:"free"`
			Inbound *int   `json:"inbound"`
			Divert  *bool  `json:"divert"`
		}
		if env.Decode(&p) == nil && p.ID != "" {
			_, err := st.PatchHospital(ctx, p.ID, p.Free, p.Inbound, p.Divert)
			return ignoreMissing(err)
		}
	case "unit.status":
		var p struct {
			ID     string  `json:"id"`
			Status string  `json:"status"`
			Task   *string `json:"task"`
			All    bool    `json:"all"`
		}
		if env.Decode(&p) != nil {
			return nil
		}
		if p.All {
			return st.AllAvailable(ctx)
		}
		if p.ID != "" && p.Status != "" {
			_, err := st.SetUnit(ctx, p.ID, p.Status, p.Task, nil)
			return ignoreMissing(err)
		}
	case "unit.progress":
		var p struct {
			ID   string `json:"id"`
			Step string `json:"step"`
		}
		if env.Decode(&p) == nil && p.ID != "" && p.Step != "" {
			_, err := st.SetUnitStep(ctx, p.ID, p.Step)
			return ignoreMissing(err)
		}
	case "shelter.occupancy", "shelter.status":
		var p struct {
			ID   string `json:"id"`
			Occ  *int   `json:"occ"`
			Open *bool  `json:"open"`
		}
		if env.Decode(&p) == nil && p.ID != "" {
			_, err := st.PatchShelter(ctx, p.ID, p.Occ, p.Open)
			return ignoreMissing(err)
		}
	case "pharmacy.stock":
		var p struct {
			ID  string `json:"id"`
			Med string `json:"med"`
			Qty int    `json:"qty"`
		}
		if env.Decode(&p) == nil && p.ID != "" && p.Med != "" {
			return ignoreMissing(st.SetStock(ctx, p.ID, p.Med, p.Qty))
		}
	case "incident.closed":
		var p struct {
			Unit string `json:"unit"`
		}
		if env.Decode(&p) == nil && p.Unit != "" {
			empty := ""
			_, err := st.SetUnit(ctx, p.Unit, "Available", &empty, nil)
			return ignoreMissing(err)
		}
	case "scenario.loaded":
		return c.Seed(ctx)
	}
	return nil
}

func ignoreMissing(err error) error {
	if err != nil && err.Error() == "not found" {
		return nil
	}
	return err
}
