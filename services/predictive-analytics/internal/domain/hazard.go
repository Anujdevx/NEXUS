// Package domain is Pūrvasūchanā's inference. hazard.go ports frontend/src/engine/hazard.js:
// which documented weak road segments would the planner treat as affected at a given rainfall.
// The thresholds are illustrative; they are not published Garhwal values.
package domain

import (
	"math"
	"sort"
)

const (
	ThreshWaterlog = 80.0  // mm: low-lying "slow" segments
	ThreshHill     = 150.0 // mm: hill "closed" segments (class m)
)

// Seg is one closure-tagged segment of the schematic network (EDGES with a closure id).
type Seg struct {
	Closure string
	Road    string
	Class   string // h | a | m
	Kind    string // closed | slow (from CLOSURES)
	Name    string
}

type Segment struct {
	Edge       string  `json:"edge"` // closure id of the segment
	Name       string  `json:"name"`
	Road       string  `json:"road"`
	Kind       string  `json:"kind"`
	Threshold  float64 `json:"threshold_mm"`
	Passable   bool    `json:"passable"`
	Confidence float64 `json:"confidence"`
}

func threshold(s Seg) (float64, bool) {
	if s.Kind == "slow" {
		return ThreshWaterlog, true
	}
	if s.Kind == "closed" && s.Class == "m" {
		return ThreshHill, true
	}
	return 0, false
}

// InferFromRain is inferFromRain(mm): the closure ids affected at this rainfall.
func InferFromRain(mm float64, segs []Seg) []string {
	set := map[string]bool{}
	for _, s := range segs {
		if s.Closure == "" {
			continue
		}
		if t, ok := threshold(s); ok && mm >= t {
			set[s.Closure] = true
		}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Segments reports every weak segment with its passability and a heuristic confidence: it grows with
// the distance from the threshold, from 0.5 at the threshold to 0.95 well past it. Inference, not observation.
func Segments(mm float64, segs []Seg) []Segment {
	seen := map[string]bool{}
	out := []Segment{}
	for _, s := range segs {
		t, ok := threshold(s)
		if !ok || seen[s.Closure] {
			continue
		}
		seen[s.Closure] = true
		affected := mm >= t
		conf := 0.5 + 0.45*math.Min(1, math.Abs(mm-t)/t)
		out = append(out, Segment{Edge: s.Closure, Name: s.Name, Road: s.Road, Kind: s.Kind, Threshold: t, Passable: !affected, Confidence: round2(conf)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Edge < out[j].Edge })
	return out
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

// Risk is the public risk band for a rainfall total.
func Risk(mm float64) string {
	switch {
	case mm >= ThreshHill:
		return "severe"
	case mm >= ThreshWaterlog:
		return "high"
	case mm >= 40:
		return "moderate"
	}
	return "low"
}
