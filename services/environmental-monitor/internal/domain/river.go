// Package domain holds environmental-monitor's rules.
package domain

// Gauge thresholds for the simulated gauges the simulator feeds. They are invented for the demo
// (CLAUDE.md data-honesty rule): every reading derived from them is labelled "simulated".
type Gauge struct {
	Warning, Danger float64 // metres
}

var Gauges = map[string]Gauge{
	"ganga-rishikesh": {Warning: 5.0, Danger: 6.0},
	"tons-tapkeshwar": {Warning: 2.5, Danger: 3.5},
	"song-maldevta":   {Warning: 2.0, Danger: 3.0},
	"rispana-city":    {Warning: 1.5, Danger: 2.5},
}

// RiverStatus classifies a level against the gauge: below | warning | danger.
func RiverStatus(gauge string, level float64) (status string, danger float64) {
	g, ok := Gauges[gauge]
	if !ok {
		g = Gauge{Warning: 3, Danger: 4}
	}
	switch {
	case level >= g.Danger:
		return "danger", g.Danger
	case level >= g.Warning:
		return "warning", g.Danger
	}
	return "below", g.Danger
}

// ClampStage keeps a flood stage in 0..100.
func ClampStage(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
