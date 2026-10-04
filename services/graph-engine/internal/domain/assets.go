package domain

import (
	"fmt"
	"math"
	"sort"

	"nexus/shared/seed"
)

type NodePos struct {
	Lat, Lng float64
	Name     string
}

// SeedTopology builds the Prāṇadhārā asset graph: hospitals and shelters from the real registry,
// plus 3 substations, 4 water pumps and 3 telecom towers, all simulated (source "simulated").
// Dependencies: pump/tower -> substation, hospital -> substation and pump (within a 3.5 km feeder zone), shelter -> tower and substation.
func SeedTopology(hospitals []seed.Hospital, shelters []seed.Shelter, nodes map[string]NodePos) *Topology {
	t := NewTopology()
	type spec struct{ id, name, node string }
	subs := []spec{{"sub-01", "Dehradun City substation", "sah"}, {"sub-02", "Rajpur–Mussoorie substation", "mdiv"}, {"sub-03", "Doiwala–Rishikesh substation", "doi"}}
	pumps := []spec{{"wp-01", "Water pump, city centre", "ct"}, {"wp-02", "Water pump, Rispana", "risp"}, {"wp-03", "Water pump, Rajpur", "mdiv"}, {"wp-04", "Water pump, Rishikesh", "rish"}}
	towers := []spec{{"twr-01", "Telecom tower, city centre", "ct"}, {"twr-02", "Telecom tower, Mussoorie", "mus"}, {"twr-03", "Telecom tower, Rishikesh", "rish"}}
	put := func(s spec, typ string, base, thr float64, grace int) {
		n := nodes[s.node]
		t.Put(Asset{ID: s.id, Name: s.name, Type: typ, Lat: n.Lat, Lng: n.Lng, Load: base, BaseLoad: base, Threshold: thr, GraceMs: grace, Source: "simulated", Node: s.node, Region: "dehradun"})
	}
	for _, s := range subs {
		put(s, "POWER", 62, 100, 5000)
	}
	for _, s := range pumps {
		put(s, "WATER", 45, 100, 4000)
	}
	for _, s := range towers {
		put(s, "TELECOM", 35, 100, 4000)
	}
	nearest := func(lat, lng float64, list []spec) string {
		best, bd := "", math.Inf(1)
		for _, s := range list {
			n := nodes[s.node]
			if d := Hav(lat, lng, n.Lat, n.Lng); d < bd {
				best, bd = s.id, d
			}
		}
		return best
	}
	dep := func(a, b string, c float64, g int) {
		t.AddDep(Dep{Dependent: a, Provider: b, Criticality: c, GraceMs: g})
	}
	for _, p := range pumps {
		n := nodes[p.node]
		dep(p.id, nearest(n.Lat, n.Lng, subs), 0.8, 4000)
	}
	for _, w := range towers {
		n := nodes[w.node]
		dep(w.id, nearest(n.Lat, n.Lng, subs), 1.0, 3000)
	}
	// Only hospitals inside a substation's feeder zone depend on it (the rest are assumed to have another feed).
	const feederKm = 3.5
	crit := map[string]float64{"Tertiary": 0.9, "Secondary": 0.6, "CHC": 0.4, "PHC": 0.4}
	within := func(lat, lng float64, list []spec) (string, bool) {
		id := nearest(lat, lng, list)
		for _, s := range list {
			if s.id == id {
				n := nodes[s.node]
				return id, Hav(lat, lng, n.Lat, n.Lng) <= feederKm
			}
		}
		return "", false
	}
	for _, h := range hospitals {
		id := "hosp-" + h.ID
		t.Put(Asset{ID: id, Name: h.N, Type: "HOSPITAL", Lat: h.Lat, Lng: h.Lng, Load: 40, BaseLoad: 40, Threshold: 100, GraceMs: 5000, Source: "simulated", Ref: h.ID, Node: h.Node, Region: "dehradun"})
		if sub, ok := within(h.Lat, h.Lng, subs); ok {
			dep(id, sub, crit[h.Lvl], 5000)
		}
		if p, ok := within(h.Lat, h.Lng, pumps); ok {
			dep(id, p, 0.4, 6000)
		}
	}
	for _, s := range shelters {
		id := "shel-" + s.ID
		t.Put(Asset{ID: id, Name: s.N, Type: "SHELTER", Lat: s.Lat, Lng: s.Lng, Load: 30, BaseLoad: 30, Threshold: 100, GraceMs: 5000, Source: "simulated", Ref: s.ID, Node: s.Node, Region: "dehradun"})
		dep(id, nearest(s.Lat, s.Lng, towers), 0.3, 6000)
		dep(id, nearest(s.Lat, s.Lng, subs), 0.3, 6000)
	}
	return t
}

// Describe is a one-line summary for logs.
func (t *Topology) Describe() string {
	counts := map[string]int{}
	for _, a := range t.List() {
		counts[a.Type]++
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	s := ""
	for _, k := range keys {
		s += fmt.Sprintf("%s=%d ", k, counts[k])
	}
	return fmt.Sprintf("%s deps=%d", s, len(t.DepList()))
}
