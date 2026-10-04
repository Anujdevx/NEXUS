package domain

import (
	"math"
	"runtime"
	"sync"
)

// This file ports frontend/src/engine/simulation.js: the connectivity sweep. The roads are real
// (or the schematic network); the SOS load, hospital capacity and connectivity are generated.
// Both policies see identical seeds.

type EvalHospital struct {
	Lat, Lng float64
	Lvl      string
}

type EvalFlood struct{ Lat, Lng float64 }

type EvalWorld struct {
	Hospitals []EvalHospital
	Flood     []EvalFlood
	BaseNodes [][2]float64 // [lat, lng] of every ambulance base
	Caps      map[string][]string
	SimCap    map[string]int
}

const evalLimit = 150.0 // minutes; beyond this a casualty counts as unserved

var EvalLevels = []float64{1, 0.8, 0.6, 0.4, 0.2}
var evalNeeds = []string{"General", "General", "Trauma", "Trauma", "Maternity", "Cardiac"}

// rng is mulberry32, bit for bit the generator in simulation.js.
func rng(seed uint32) func() float64 {
	a := seed
	return func() float64 {
		a += 0x6d2b79f5
		t := (a ^ (a >> 15)) * (1 | a)
		t = (t + (t^(t>>7))*(61|t)) ^ t
		return float64(t^(t>>14)) / 4294967296
	}
}

type pathInfo struct {
	plain, min float64
	cut        int
	tcut       float64
}

type srcTable struct {
	full  []float64
	blind []pathInfo
}

type tables struct {
	net        *Net
	w          EvalWorld
	blocked    map[int]bool
	slow       map[int]bool
	hosp, orig []int
	bases      []int
	dest       []int
	mu         sync.Mutex
	T          map[int]*srcTable
	sources    []int
}

func newTables(n *Net, w EvalWorld, blocked, slow map[int]bool) *tables {
	t := &tables{net: n, w: w, blocked: blocked, slow: slow, T: map[int]*srcTable{}}
	for _, h := range w.Hospitals {
		t.hosp = append(t.hosp, n.NearestNode(h.Lat, h.Lng))
	}
	for _, f := range w.Flood {
		t.orig = append(t.orig, n.NearestNode(f.Lat, f.Lng))
	}
	for _, b := range w.BaseNodes {
		t.bases = append(t.bases, n.NearestNode(b[0], b[1]))
	}
	t.dest = append(append([]int{}, t.hosp...), t.orig...)
	seen := map[int]bool{}
	for _, list := range [][]int{t.orig, t.bases, t.hosp} {
		for _, s := range list {
			if !seen[s] {
				seen[s] = true
				t.sources = append(t.sources, s)
			}
		}
	}
	return t
}

func (t *tables) costFull(e *Edge) float64 {
	if t.blocked[e.I] {
		return math.Inf(1)
	}
	if t.slow[e.I] {
		return e.Min * TrueSlow
	}
	return e.Min
}

func (t *tables) compute(src int) *srcTable {
	f := t.net.TreeFrom(src, t.costFull)
	b := t.net.TreeFrom(src, func(e *Edge) float64 { return e.Min })
	st := &srcTable{full: make([]float64, len(t.dest)), blind: make([]pathInfo, len(t.dest))}
	for i, d := range t.dest {
		st.full[i] = f.Dist[d]
		if math.IsInf(b.Dist[d], 1) {
			st.blind[i] = pathInfo{math.Inf(1), math.Inf(1), -1, 0}
			continue
		}
		var es, ns []int
		ns = append(ns, d)
		for v := d; v != src; v = b.Prev[v] {
			es = append([]int{b.Via[v]}, es...)
			ns = append([]int{b.Prev[v]}, ns...)
		}
		min := 0.0
		cut := -2
		var tcut float64
		for k := range es {
			if t.blocked[es[k]] {
				cut, tcut = ns[k], min
				break
			}
			if t.slow[es[k]] {
				min += t.net.Edges[es[k]].Min * TrueSlow
			} else {
				min += t.net.Edges[es[k]].Min
			}
		}
		if cut == -2 {
			st.blind[i] = pathInfo{b.Dist[d], min, -1, 0}
		} else {
			st.blind[i] = pathInfo{b.Dist[d], math.Inf(1), cut, tcut}
		}
	}
	return st
}

func (t *tables) ensure(src int) *srcTable {
	t.mu.Lock()
	if st, ok := t.T[src]; ok {
		t.mu.Unlock()
		return st
	}
	t.mu.Unlock()
	st := t.compute(src)
	t.mu.Lock()
	t.T[src] = st
	t.mu.Unlock()
	return st
}

// prepare fills the tables for every source using all CPUs.
func (t *tables) prepare() {
	jobs := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s := range jobs {
				t.ensure(s)
			}
		}()
	}
	for _, s := range t.sources {
		jobs <- s
	}
	close(jobs)
	wg.Wait()
}

// trip: minutes actually driven. A crew without the road picture drives into the break, loses three minutes, and replans.
func (t *tables) trip(src, di int, knows bool) float64 {
	st := t.ensure(src)
	if knows {
		return st.full[di]
	}
	b := st.blind[di]
	if b.cut < 0 {
		return b.min
	}
	return b.tcut + 3 + t.ensure(b.cut).full[di]
}

func (t *tables) guess(src, di int, knows bool) float64 {
	st := t.ensure(src)
	if knows {
		return st.full[di]
	}
	return st.blind[di].plain
}

type runResult struct {
	mean, unserved, quality, delivery float64
}

func (t *tables) oneRun(seed uint32, conn float64, policy string, nSos int, relay, retry float64) runResult {
	rand := rng(seed)
	expo := func(mean float64) float64 { return -math.Log(1-rand()) * mean }
	H := t.w.Hospitals
	free := make([]int, len(H))
	for i, h := range H {
		free[i] = int(math.Max(1, math.Round(float64(t.w.SimCap[h.Lvl])*(0.08+rand()*0.22))))
	}
	var sum float64
	served, unserved, good, delivered := 0, 0, 0, 0
	for i := 0; i < nSos; i++ {
		oi := int(math.Floor(rand() * float64(len(t.orig))))
		origin, od := t.orig[oi], len(H)+oi
		need := evalNeeds[int(math.Floor(rand()*float64(len(evalNeeds))))]
		online := rand() < conn
		rKnow, rCap := rand(), rand()
		dN, dB := expo(relay), expo(retry)
		delay := 0.0
		if !online {
			if policy == "nexus" {
				delay = dN
			} else {
				delay = dB
			}
		}
		if delay <= 30 {
			delivered++
		}
		knows := policy == "nexus" && rKnow < conn
		seesCapacity := policy == "nexus" && rCap < conn

		base, bb := t.bases[0], math.Inf(1)
		for _, b := range t.bases {
			var c float64
			if policy == "nexus" {
				c = t.guess(b, od, knows)
			} else {
				c = Hav(t.net.Lat[b], t.net.Lng[b], t.net.Lat[origin], t.net.Lng[origin])
			}
			if c < bb {
				bb, base = c, b
			}
		}
		tt := delay + t.trip(base, od, knows)

		tried := map[int]bool{}
		at, ok, first := origin, false, true
		for hop := 0; hop < 4 && !math.IsInf(tt, 1) && !math.IsNaN(tt); hop++ {
			pick, best := -1, math.Inf(1)
			for hi, h := range H {
				if tried[hi] {
					continue
				}
				var c float64
				if policy == "nexus" {
					if !contains(t.w.Caps[h.Lvl], need) {
						continue
					}
					c = t.guess(at, hi, knows)
					if seesCapacity || hop > 0 {
						if free[hi] > 0 {
							c += 0
						} else {
							c += 500
						}
					}
				} else {
					c = Hav(t.net.Lat[at], t.net.Lng[at], h.Lat, h.Lng)
				}
				if c < best {
					best, pick = c, hi
				}
			}
			if pick < 0 {
				break
			}
			tried[pick] = true
			tt += t.trip(at, pick, knows)
			fits := contains(t.w.Caps[H[pick].Lvl], need) && free[pick] > 0
			if first && fits {
				good++
			}
			first = false
			if fits {
				free[pick]--
				ok = true
				break
			}
			tt += 6
			at = t.hosp[pick]
		}
		if ok && tt <= evalLimit {
			sum += tt
			served++
		} else {
			unserved++
		}
	}
	mean := math.NaN()
	if served > 0 {
		mean = sum / float64(served)
	}
	return runResult{mean, float64(unserved), float64(good) / float64(nSos), float64(delivered) / float64(nSos)}
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

type Stat struct {
	M  *float64 `json:"m"`
	CI *float64 `json:"ci"`
}

func stats(xs []float64) Stat {
	var v []float64
	for _, x := range xs {
		if !math.IsNaN(x) && !math.IsInf(x, 0) {
			v = append(v, x)
		}
	}
	n := float64(len(v))
	if n == 0 {
		n = 1
	}
	var m float64
	for _, x := range v {
		m += x
	}
	m /= n
	var ss float64
	for _, x := range v {
		ss += (x - m) * (x - m)
	}
	sd := math.Sqrt(ss / math.Max(1, n-1))
	ci := 1.96 * sd / math.Sqrt(n)
	if len(v) == 0 {
		return Stat{} // JS would give NaN; null on the wire
	}
	return Stat{&m, &ci}
}

type EvalRow struct {
	Conn     float64 `json:"conn"`
	Policy   string  `json:"policy"`
	TTC      Stat    `json:"ttc"`
	Unserved Stat    `json:"unserved"`
	Quality  Stat    `json:"quality"`
	Delivery Stat    `json:"delivery"`
}

type EvalParams struct {
	NSos  int     `json:"nSos"`
	Runs  int     `json:"runs"`
	Seed  uint32  `json:"seed"`
	Relay float64 `json:"relay"`
	Retry float64 `json:"retry"`
}

func DefaultEval() EvalParams {
	return EvalParams{NSos: 40, Runs: 30, Seed: 2026, Relay: 12, Retry: 25}
}

// Sweep runs both policies at every connectivity level (sweep() in simulation.js).
func Sweep(n *Net, w EvalWorld, blocked, slow map[int]bool, p EvalParams) []EvalRow {
	tb := newTables(n, w, blocked, slow)
	tb.prepare()
	type job struct {
		c float64
		p string
	}
	var jobs []job
	for _, c := range EvalLevels {
		for _, pol := range []string{"nexus", "base"} {
			jobs = append(jobs, job{c, pol})
		}
	}
	out := make([]EvalRow, len(jobs))
	var wg sync.WaitGroup
	for j, jb := range jobs {
		wg.Add(1)
		go func(j int, jb job) {
			defer wg.Done()
			var ttc, uns, q, d []float64
			for r := 0; r < p.Runs; r++ {
				x := tb.oneRun(p.Seed+uint32(r)*7919, jb.c, jb.p, p.NSos, p.Relay, p.Retry)
				ttc, uns, q, d = append(ttc, x.mean), append(uns, x.unserved), append(q, x.quality), append(d, x.delivery)
			}
			out[j] = EvalRow{jb.c, jb.p, stats(ttc), stats(uns), stats(q), stats(d)}
		}(j, jb)
	}
	wg.Wait()
	return out
}
