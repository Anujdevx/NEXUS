package domain

import "encoding/json"

// BuiltinWays ports builtin(): the schematic network from NODES/EDGES (frontend/src/data/index.js).
// nodes: id -> [lat, lng, name]; edges: [from, to, road, class, closureId?]; speed and wind by class.
func BuiltinWays(nodes map[string][]json.RawMessage, edges [][]json.RawMessage, speed, wind map[string]float64) []Way {
	cls := map[string]string{"h": "primary", "a": "secondary", "m": "tertiary"}
	str := func(r json.RawMessage) string { var s string; _ = json.Unmarshal(r, &s); return s }
	num := func(r json.RawMessage) float64 { var f float64; _ = json.Unmarshal(r, &f); return f }
	ll := func(id string) []float64 { n := nodes[id]; return []float64{num(n[0]), num(n[1])} }
	var ways []Way
	for _, e := range edges {
		a, b, road, c := str(e[0]), str(e[1]), str(e[2]), str(e[3])
		w := Way{Name: road, HW: cls[c], Pts: [][]float64{ll(a), ll(b)}, Kmh: speed[c], Wind: wind[c]}
		if len(e) > 4 {
			w.CID = str(e[4])
		}
		ways = append(ways, w)
	}
	return ways
}
