package domain

import (
	"sort"
	"sync"
	"time"
)

const (
	Active   = "ACTIVE"
	Degraded = "DEGRADED"
	Failed   = "FAILED"
)

type Asset struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Type      string  `json:"type"` // POWER | WATER | TELECOM | HOSPITAL | SHELTER | PUMP | BRIDGE
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Status    string  `json:"status"`
	Load      float64 `json:"load_percentage"`
	Threshold float64 `json:"failure_threshold"`
	GraceMs   int     `json:"grace_period_ms"`
	Source    string  `json:"source"`         // "simulated" for utilities and the dependency graph
	Ref       string  `json:"ref,omitempty"`  // id of the real record (hospital or shelter) this asset stands for
	Node      string  `json:"node,omitempty"` // nearest junction id (NEAR relationship)
	Region    string  `json:"region,omitempty"`
	BaseLoad  float64 `json:"base_load"`
	overSince time.Time
}

// Dep is (dependent)-[:DEPENDS_ON {criticality, grace_period_ms}]->(provider).
type Dep struct {
	Dependent   string  `json:"dependent"`
	Provider    string  `json:"provider"`
	Criticality float64 `json:"criticality"`
	GraceMs     int     `json:"grace_period_ms"`
}

type Topology struct {
	mu     sync.RWMutex
	Assets map[string]*Asset
	Deps   []Dep
	order  []string
}

func NewTopology() *Topology { return &Topology{Assets: map[string]*Asset{}} }

func (t *Topology) Put(a Asset) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if a.Status == "" {
		a.Status = Active
	}
	if a.BaseLoad == 0 {
		a.BaseLoad = a.Load
	}
	if _, ok := t.Assets[a.ID]; !ok {
		t.order = append(t.order, a.ID)
	}
	c := a
	t.Assets[a.ID] = &c
}

func (t *Topology) AddDep(d Dep) {
	t.mu.Lock()
	t.Deps = append(t.Deps, d)
	t.mu.Unlock()
}

func (t *Topology) List() []Asset {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]Asset, 0, len(t.order))
	for _, id := range t.order {
		out = append(out, *t.Assets[id])
	}
	return out
}

func (t *Topology) DepList() []Dep {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return append([]Dep(nil), t.Deps...)
}

func (t *Topology) Get(id string) (Asset, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	a, ok := t.Assets[id]
	if !ok {
		return Asset{}, false
	}
	return *a, true
}

type Cascade struct {
	Root     string     `json:"root"`
	Waves    [][]string `json:"waves"`
	Failed   []string   `json:"failed"`
	Degraded []string   `json:"degraded"`
	Changed  []Asset    `json:"-"`
}

// Fail runs the cascade (CLAUDE.md §7.2) from root. With apply=false it works on a copy (blast radius).
//  1. root becomes FAILED.
//  2. BFS outwards over the reverse of DEPENDS_ON: for each failed f, every dependent d with an edge d->f.
//  3. d.load += 100*criticality. load > threshold: d FAILED and joins the next wave. Otherwise DEGRADED.
//  4. each BFS level is a wave.
func (t *Topology) Fail(root string, apply bool) (Cascade, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	r, ok := t.Assets[root]
	if !ok {
		return Cascade{}, false
	}
	work := map[string]*Asset{}
	get := func(id string) *Asset {
		if apply {
			return t.Assets[id]
		}
		if w, ok := work[id]; ok {
			return w
		}
		c := *t.Assets[id]
		work[id] = &c
		return &c
	}
	byProvider := map[string][]Dep{}
	for _, d := range t.Deps {
		byProvider[d.Provider] = append(byProvider[d.Provider], d)
	}
	touched := map[string]bool{root: true}
	res := Cascade{Root: root, Waves: [][]string{}, Failed: []string{root}, Degraded: []string{}}
	get(root).Status = Failed
	level := []string{root}
	for len(level) > 0 {
		var next, wave []string
		for _, f := range level {
			for _, d := range byProvider[f] {
				a := get(d.Dependent)
				if a == nil || a.Status == Failed {
					continue
				}
				a.Load += 100 * d.Criticality
				touched[a.ID] = true
				if a.Load > a.Threshold {
					a.Status = Failed
					next = append(next, a.ID)
					res.Failed = append(res.Failed, a.ID)
					res.Degraded = remove(res.Degraded, a.ID)
				} else if a.Status != Degraded {
					a.Status = Degraded
					res.Degraded = append(res.Degraded, a.ID)
				}
				wave = appendUnique(wave, a.ID)
			}
		}
		if len(wave) > 0 {
			res.Waves = append(res.Waves, wave)
		}
		level = next
	}
	ids := make([]string, 0, len(touched))
	for id := range touched {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		res.Changed = append(res.Changed, *get(id))
	}
	_ = r
	return res, true
}

// Restore returns one asset to ACTIVE at its base load.
func (t *Topology) Restore(id string) (Asset, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	a, ok := t.Assets[id]
	if !ok {
		return Asset{}, false
	}
	a.Status, a.Load, a.overSince = Active, a.BaseLoad, time.Time{}
	return *a, true
}

// Reset returns every asset to ACTIVE; it reports the ones that changed.
func (t *Topology) Reset() []Asset {
	t.mu.Lock()
	defer t.mu.Unlock()
	var ch []Asset
	for _, id := range t.order {
		a := t.Assets[id]
		if a.Status != Active || a.Load != a.BaseLoad {
			a.Status, a.Load, a.overSince = Active, a.BaseLoad, time.Time{}
			ch = append(ch, *a)
		}
	}
	return ch
}

// Telemetry records a load reading. It reports true when the asset has been over its failure
// threshold for longer than its grace period and is not already failed.
func (t *Topology) Telemetry(id string, load float64, now time.Time) (trip bool, known bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	a, ok := t.Assets[id]
	if !ok {
		return false, false
	}
	if a.Status == Failed {
		return false, true
	}
	a.Load = load
	if load > a.Threshold {
		if a.overSince.IsZero() {
			a.overSince = now
		}
		if now.Sub(a.overSince) >= time.Duration(a.GraceMs)*time.Millisecond {
			a.overSince = time.Time{}
			return true, true
		}
		return false, true
	}
	a.overSince = time.Time{}
	return false, true
}

func remove(l []string, s string) []string {
	out := l[:0]
	for _, x := range l {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

func appendUnique(l []string, s string) []string {
	for _, x := range l {
		if x == s {
			return l
		}
	}
	return append(l, s)
}

// Replace swaps in another topology's contents (used after loading from Neo4j).
func (t *Topology) Replace(o *Topology) {
	t.mu.Lock()
	defer t.mu.Unlock()
	o.mu.RLock()
	defer o.mu.RUnlock()
	t.Assets, t.Deps, t.order = o.Assets, o.Deps, o.order
}
