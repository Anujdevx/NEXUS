package domain

import (
	"fmt"
	"math"
	"sync"

	"nexus/shared/seed"
)

// RoadState is the live road picture the router plans against: documented closures,
// reported breaks, and the illustrative flood stage. It mirrors the frontend's
// state.closures / state.breaks / state.flood.stage and is fed by bus events.
type RoadState struct {
	mu       sync.RWMutex
	Closures map[string]bool
	Breaks   []Point
	Flood    float64
	Conf     float64

	floodKey  string
	floodBlk  map[int]bool
	floodSlow map[int]bool
}

func NewRoadState(defaultClosures []string) *RoadState {
	s := &RoadState{Closures: map[string]bool{}, Conf: 0.9}
	s.Reset(defaultClosures)
	return s
}

func (s *RoadState) Reset(closures []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Closures = map[string]bool{}
	for _, c := range closures {
		s.Closures[c] = true
	}
	s.Breaks, s.Flood, s.floodKey = nil, 0, ""
}

func (s *RoadState) SetClosure(id string, on bool) {
	s.mu.Lock()
	s.Closures[id] = on
	s.mu.Unlock()
}

func (s *RoadState) AddBreak(p Point) {
	s.mu.Lock()
	s.Breaks = append(s.Breaks, p)
	s.mu.Unlock()
}

// RemoveBreak drops the break nearest the given point (the frontend removes by list index).
func (s *RoadState) RemoveBreak(lat, lng float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	best, bd := -1, math.Inf(1)
	for i, b := range s.Breaks {
		if d := Hav(lat, lng, b.Lat, b.Lng); d < bd {
			best, bd = i, d
		}
	}
	if best >= 0 && bd < 0.05 {
		s.Breaks = append(s.Breaks[:best], s.Breaks[best+1:]...)
	}
}

func (s *RoadState) ClearBreaks() { s.mu.Lock(); s.Breaks = nil; s.mu.Unlock() }
func (s *RoadState) SetFlood(stage float64) {
	s.mu.Lock()
	s.Flood = stage
	s.mu.Unlock()
}

type Snapshot struct {
	Closures []string `json:"closures"`
	Breaks   []Point  `json:"breaks"`
	Flood    float64  `json:"flood"`
	Conf     float64  `json:"conf"`
}

func (s *RoadState) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := Snapshot{Closures: []string{}, Breaks: append([]Point{}, s.Breaks...), Flood: s.Flood, Conf: s.Conf}
	for id, on := range s.Closures {
		if on {
			out.Closures = append(out.Closures, id)
		}
	}
	return out
}

// Planner binds the network to the closure/flood datasets so requests can be resolved to edge sets.
type Planner struct {
	Net      *Net
	Kinds    map[string]string
	Flood    []seed.Flood
	State    *RoadState
	floodMu  sync.Mutex
	floodKey string
	fBlk     map[int]bool
	fSlow    map[int]bool
}

// floodSets ports floodState(): water rises around the documented flood-prone localities.
func (p *Planner) floodSets(stage float64) (map[int]bool, map[int]bool) {
	p.floodMu.Lock()
	defer p.floodMu.Unlock()
	key := stageKey(p.Net.Version, stage)
	if p.floodKey == key {
		return p.fBlk, p.fSlow
	}
	blocked, slow := map[int]bool{}, map[int]bool{}
	if stage > 0 {
		for i, f := range p.Flood {
			onset := float64(4 + (i*37)%56)
			r := math.Max(0, math.Min(1, (stage-onset)/40)) * 0.6
			if r <= 0.02 {
				continue
			}
			for _, e := range p.Net.EdgesNear(f.Lat, f.Lng, r, true) {
				slow[e] = true
			}
			for _, e := range p.Net.EdgesNear(f.Lat, f.Lng, r*0.55, true) {
				blocked[e] = true
				delete(slow, e)
			}
		}
	}
	p.floodKey, p.fBlk, p.fSlow = key, blocked, slow
	return blocked, slow
}

func stageKey(v int, s float64) string { return fmt.Sprintf("%d|%.2f", v, s) }

// Override lets one request supply its own picture instead of the live state.
type Override struct {
	Closures *[]string
	Breaks   *[]Point
	BreakIDs []int
	Flood    *float64
	Conf     *float64
	Hard     bool
}

// Context resolves the live state (or the override) into the planner's Ctx: documented closures,
// reported breaks and flood water, exactly as currentSets() + netCtx() do in store.js.
func (p *Planner) Context(o Override) Ctx {
	snap := p.State.Snapshot()
	closures, breaks, flood, conf := snap.Closures, snap.Breaks, snap.Flood, snap.Conf
	if o.Closures != nil {
		closures = *o.Closures
	}
	if o.Breaks != nil {
		breaks = *o.Breaks
	}
	if o.Flood != nil {
		flood = *o.Flood
	}
	if o.Conf != nil {
		conf = *o.Conf
	}
	blocked, slow := p.Net.Resolve(closures, p.Kinds, breaks)
	for _, e := range o.BreakIDs {
		blocked[e] = true
	}
	fb, fs := p.floodSets(flood)
	for e := range fb {
		blocked[e] = true
		delete(slow, e)
	}
	for e := range fs {
		if !blocked[e] {
			slow[e] = true
		}
	}
	return Ctx{Blocked: blocked, Slow: slow, Conf: conf, Hard: o.Hard}
}
