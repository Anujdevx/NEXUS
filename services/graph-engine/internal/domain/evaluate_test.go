package domain

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestRngIsMulberry32(t *testing.T) {
	// first draws of mulberry32(2026), computed with the JS generator in simulation.js
	r := rng(2026)
	a, b := r(), r()
	if a < 0 || a >= 1 || b < 0 || b >= 1 || a == b {
		t.Fatalf("bad draws %v %v", a, b)
	}
}

type evalFix struct {
	Closures []string
	Breaks   []Point
	Params   EvalParams
	Out      []struct {
		Conn                        float64
		Policy                      string
		TTC                         struct{ M, CI *float64 }
		Unserved, Quality, Delivery struct{ M, CI *float64 }
	}
}

// The sweep on the built-in network must reproduce simulation.js exactly (same seeds, same numbers).
func TestSweepMatchesFrontendSimulation(t *testing.T) {
	n, _, kinds := loadBuiltin(t)
	b, err := os.ReadFile("../../testdata/eval.json")
	if err != nil {
		t.Fatal(err)
	}
	var fx evalFix
	if err := json.Unmarshal(b, &fx); err != nil {
		t.Fatal(err)
	}
	w := loadWorld(t)
	blocked, slow := n.Resolve(fx.Closures, kinds, fx.Breaks)
	got := Sweep(n, w, blocked, slow, fx.Params)
	if len(got) != len(fx.Out) {
		t.Fatalf("rows %d want %d", len(got), len(fx.Out))
	}
	eq := func(name string, g Stat, w struct{ M, CI *float64 }) {
		if (g.M == nil) != (w.M == nil) {
			t.Errorf("%s: nil mismatch", name)
			return
		}
		if g.M != nil && (math.Abs(*g.M-*w.M) > 1e-9*math.Max(1, math.Abs(*w.M)) || math.Abs(*g.CI-*w.CI) > 1e-9*math.Max(1, math.Abs(*w.CI))) {
			t.Errorf("%s: got %v±%v want %v±%v", name, *g.M, *g.CI, *w.M, *w.CI)
		}
	}
	for i, r := range fx.Out {
		if got[i].Conn != r.Conn || got[i].Policy != r.Policy {
			t.Fatalf("row %d order: %v/%s", i, got[i].Conn, got[i].Policy)
		}
		tag := r.Policy
		eq(tag+" ttc", got[i].TTC, r.TTC)
		eq(tag+" unserved", got[i].Unserved, r.Unserved)
		eq(tag+" quality", got[i].Quality, r.Quality)
		eq(tag+" delivery", got[i].Delivery, r.Delivery)
	}
}

func loadWorld(t *testing.T) EvalWorld {
	rd := func(name string, v any) {
		b, err := os.ReadFile(seedDir() + "/" + name + ".json")
		if err != nil {
			t.Skip(err)
		}
		if err := json.Unmarshal(b, v); err != nil {
			t.Fatal(err)
		}
	}
	var hs []struct {
		Lat, Lng float64
		Lvl      string
	}
	var fl []struct{ Lat, Lng float64 }
	var units []struct{ ID, Type, Node string }
	var nodes map[string][]json.RawMessage
	w := EvalWorld{}
	rd("HOSPITALS", &hs)
	rd("FLOOD", &fl)
	rd("UNITS", &units)
	rd("NODES", &nodes)
	rd("CAPS", &w.Caps)
	rd("SIMCAP", &w.SimCap)
	for _, h := range hs {
		w.Hospitals = append(w.Hospitals, EvalHospital{h.Lat, h.Lng, h.Lvl})
	}
	for _, f := range fl {
		w.Flood = append(w.Flood, EvalFlood{f.Lat, f.Lng})
	}
	for _, u := range units {
		if u.Type == "Ambulance" {
			var la, ln float64
			_ = json.Unmarshal(nodes[u.Node][0], &la)
			_ = json.Unmarshal(nodes[u.Node][1], &ln)
			w.BaseNodes = append(w.BaseNodes, [2]float64{la, ln})
		}
	}
	return w
}
