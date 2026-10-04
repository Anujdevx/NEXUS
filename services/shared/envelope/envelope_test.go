package envelope

import (
	"encoding/json"
	"testing"
)

func TestKeyAndRoundTrip(t *testing.T) {
	e := New("sos", "request", "ahvana", "citizen-reporting").WithGeo(30.3, 78.0).WithStatus("direct").WithCapacity(map[string]any{"people": 4}).WithPayload(map[string]any{"id": "SOS-1"})
	if e.Key() != "sos.request" || e.ID == "" || e.Confidence != 1 {
		t.Fatalf("%+v", e)
	}
	b, _ := json.Marshal(e)
	var back Envelope
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	var p struct{ ID string }
	if err := back.Decode(&p); err != nil || p.ID != "SOS-1" {
		t.Fatalf("payload: %v %v", p, err)
	}
	if back.Status == nil || *back.Status != "direct" || back.Geo[0] != 30.3 {
		t.Fatal("fields lost in round trip")
	}
}

func TestFillSuppliesDefaults(t *testing.T) {
	var e Envelope
	e.Entity, e.Type = "hospital", "capacity"
	e.Fill("web")
	if e.ID == "" || e.Timestamp == "" || e.Origin != "web" || e.Source != "unknown" {
		t.Fatalf("%+v", e)
	}
	var null Envelope
	if err := null.Decode(&struct{}{}); err != nil {
		t.Fatal("decoding an empty payload must not fail")
	}
}
