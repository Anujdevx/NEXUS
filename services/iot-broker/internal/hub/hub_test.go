package hub

import (
	"testing"

	"nexus/shared/auth"
	"nexus/shared/envelope"
)

func env(entity, typ, origin, payload string) envelope.Envelope {
	e := envelope.New(entity, typ, "test", origin)
	e.Payload = []byte(payload)
	return e
}

func TestRoleFilterAndEchoDrop(t *testing.T) {
	ctl := &Client{ID: "web-ctl", Role: auth.Controller}
	resp := &Client{ID: "web-r", Role: auth.Responder, Unit: "AMB-04"}
	cit := &Client{ID: "web-c", Role: auth.Citizen}
	cases := []struct {
		name string
		e    envelope.Envelope
		c    *Client
		want bool
	}{
		{"controller gets everything", env("hospital", "capacity", "svc", `{}`), ctl, true},
		{"echo is dropped", env("sos", "request", "web-ctl", `{}`), ctl, false},
		{"responder gets road state", env("road_segment", "passability", "svc", `{}`), resp, true},
		{"responder gets its own unit", env("unit", "status", "svc", `{"id":"AMB-04"}`), resp, true},
		{"responder skips another unit", env("unit", "status", "svc", `{"id":"AMB-07"}`), resp, false},
		{"responder skips hospital capacity", env("hospital", "capacity", "svc", `{}`), resp, false},
		{"citizen gets public alerts", env("public_alert", "cap_alert", "svc", `{}`), cit, true},
		{"citizen gets its own assignment", env("assignment", "dispatched", "svc", `{"client":"web-c"}`), cit, true},
		{"citizen skips someone else's sos", env("sos", "request", "svc", `{"client":"web-x"}`), cit, false},
		{"citizen skips hospital capacity", env("hospital", "capacity", "svc", `{}`), cit, false},
		{"citizen skips an sos with no owner", env("sos", "request", "svc", `{}`), cit, false},
	}
	for _, c := range cases {
		if got := c.c.wants(c.e); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
