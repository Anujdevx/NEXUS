// Package domain holds graph-engine's pure logic. roadnet.go is an exact port of
// frontend/src/engine/roadnet.js so backend and offline routes are identical:
// same graph build, same grid lookups, same heap, same A* tie-breaking.
package domain

import (
	"math"
	"strconv"
)

const (
	hard     = 40.0
	slowK    = 1.2
	TrueSlow = 2.2
	cell     = 0.01
	rad      = math.Pi / 180
)

var kmh = map[string]float64{"motorway": 60, "trunk": 50, "primary": 38, "secondary": 32, "tertiary": 26, "unclassified": 20, "residential": 16}
var className = map[string]string{"motorway": "Motorway", "trunk": "Trunk road", "primary": "Primary road", "secondary": "Secondary road", "tertiary": "Tertiary road", "unclassified": "Minor road", "residential": "Residential street"}

func Hav(aLat, aLng, bLat, bLng float64) float64 {
	dLat, dLng := (bLat-aLat)*rad, (bLng-aLng)*rad
	s := math.Pow(math.Sin(dLat/2), 2) + math.Cos(aLat*rad)*math.Cos(bLat*rad)*math.Pow(math.Sin(dLng/2), 2)
	return 12742 * math.Asin(math.Sqrt(s))
}

type Edge struct {
	I, A, B  int
	Km, Min  float64
	Name, HW string
	Way      int
}

// Way is one road polyline. IDs (OSM node ids) are optional; without them nodes merge on 5-decimal coordinates.
type Way struct {
	Name string      `json:"name"`
	HW   string      `json:"hw"`
	IDs  []int64     `json:"ids"`
	Pts  [][]float64 `json:"pts"`
	Kmh  float64     `json:"-"`
	Wind float64     `json:"-"`
	CID  string      `json:"-"`
}

type ClosureAt struct {
	Kind string
	At   [2]float64
}

type Net struct {
	Source  string
	Lat     []float64
	Lng     []float64
	Adj     [][]*Edge
	Edges   []*Edge
	grid    map[[2]int][]int
	CID     map[string][]int
	Version int
}

type Info struct {
	Source string `json:"source"`
	Nodes  int    `json:"nodes"`
	Edges  int    `json:"edges"`
	Label  string `json:"label"`
}

func (n *Net) Describe() Info {
	l := "OpenStreetMap roads (saved file)"
	switch n.Source {
	case "builtin":
		l = "Built-in schematic network"
	case "osm-live":
		l = "OpenStreetMap roads (live from Overpass)"
	}
	return Info{n.Source, len(n.Lat), len(n.Edges), l}
}

// Build ports build(): closures map is used for non-builtin sources to find the edges near each closure point.
func Build(ways []Way, source string, closures map[string]ClosureAt, version int) *Net {
	n := &Net{Source: source, grid: map[[2]int][]int{}, CID: map[string][]int{}, Version: version}
	index := map[string]int{}
	node := func(key string, la, ln float64) int {
		if i, ok := index[key]; ok {
			return i
		}
		i := len(n.Lat)
		index[key] = i
		n.Lat = append(n.Lat, la)
		n.Lng = append(n.Lng, ln)
		n.Adj = append(n.Adj, nil)
		g := [2]int{int(math.Floor(la / cell)), int(math.Floor(ln / cell))}
		n.grid[g] = append(n.grid[g], i)
		return i
	}
	for wi, w := range ways {
		sp := w.Kmh
		if sp == 0 {
			sp = kmh[w.HW]
		}
		if sp == 0 {
			sp = 20
		}
		wind := w.Wind
		if wind == 0 {
			wind = 1
		}
		name := w.Name
		if name == "" {
			name = className[w.HW]
		}
		if name == "" {
			name = "Road"
		}
		prev := -1
		for k, p := range w.Pts {
			var key string
			if w.IDs != nil && k < len(w.IDs) {
				key = strconv.FormatInt(w.IDs[k], 10)
			} else {
				key = strconv.FormatFloat(p[0], 'f', 5, 64) + "," + strconv.FormatFloat(p[1], 'f', 5, 64)
			}
			c := node(key, p[0], p[1])
			if prev >= 0 && prev != c {
				km := Hav(n.Lat[prev], n.Lng[prev], p[0], p[1]) * wind
				e := &Edge{I: len(n.Edges), A: prev, B: c, Km: km, Min: km / sp * 60, Name: name, HW: w.HW, Way: wi}
				n.Edges = append(n.Edges, e)
				n.Adj[prev] = append(n.Adj[prev], e)
				n.Adj[c] = append(n.Adj[c], e)
				if w.CID != "" {
					n.CID[w.CID] = append(n.CID[w.CID], e.I)
				}
			}
			prev = c
		}
	}
	if source != "builtin" {
		for id, c := range closures {
			n.CID[id] = n.EdgesNear(c.At[0], c.At[1], 0.15, false)
		}
	}
	return n
}

func (n *Net) nearCells(la, ln float64, ring int) []int {
	var out []int
	ci, cj := int(math.Floor(la/cell)), int(math.Floor(ln/cell))
	for i := ci - ring; i <= ci+ring; i++ {
		for j := cj - ring; j <= cj+ring; j++ {
			out = append(out, n.grid[[2]int{i, j}]...)
		}
	}
	return out
}

func (n *Net) NearestNode(la, ln float64) int {
	for _, ring := range []int{1, 3, 8, 25, 80} {
		c := n.nearCells(la, ln, ring)
		if len(c) == 0 {
			continue
		}
		best, bd := c[0], math.Inf(1)
		for _, i := range c {
			if d := Hav(la, ln, n.Lat[i], n.Lng[i]); d < bd {
				bd, best = d, i
			}
		}
		return best
	}
	return 0
}

func (n *Net) segDist(la, ln float64, e *Edge) float64 {
	ax, ay, bx, by := n.Lng[e.A], n.Lat[e.A], n.Lng[e.B], n.Lat[e.B]
	k := math.Cos(la * rad)
	px, py, dx, dy := (ln-ax)*k, la-ay, (bx-ax)*k, by-ay
	den := dx*dx + dy*dy
	if den == 0 {
		den = 1
	}
	t := math.Max(0, math.Min(1, (px*dx+py*dy)/den))
	return Hav(la, ln, ay+t*(by-ay), ax+t*(bx-ax))
}

func (n *Net) EdgesNear(la, ln, km float64, strict bool) []int {
	seen := map[int]bool{}
	var out []int
	ring := int(math.Max(2, math.Ceil(km/1.0)+1))
	for _, nd := range n.nearCells(la, ln, ring) {
		for _, e := range n.Adj[nd] {
			if !seen[e.I] {
				seen[e.I] = true
				if n.segDist(la, ln, e) <= km {
					out = append(out, e.I)
				}
			}
		}
	}
	if len(out) == 0 && !strict {
		if e, _ := n.NearestEdge(la, ln); e != nil {
			out = append(out, e.I)
		}
	}
	return out
}

func (n *Net) NearestEdge(la, ln float64) (*Edge, float64) {
	var best *Edge
	bd := math.Inf(1)
	for _, ring := range []int{1, 4, 12, 40} {
		for _, nd := range n.nearCells(la, ln, ring) {
			for _, e := range n.Adj[nd] {
				if d := n.segDist(la, ln, e); d < bd {
					bd, best = d, e
				}
			}
		}
		if best != nil {
			break
		}
	}
	return best, bd
}

func (n *Net) EdgeMid(e *Edge) (float64, float64) {
	return (n.Lat[e.A] + n.Lat[e.B]) / 2, (n.Lng[e.A] + n.Lng[e.B]) / 2
}

// ---- binary heap (same sift rules as the JS Heap, so ties resolve identically) ----

type heap struct {
	k []float64
	v []int
}

func (h *heap) push(key float64, val int) {
	i := len(h.k)
	h.k = append(h.k, key)
	h.v = append(h.v, val)
	for i > 0 {
		p := (i - 1) >> 1
		if h.k[p] <= h.k[i] {
			break
		}
		h.k[p], h.k[i] = h.k[i], h.k[p]
		h.v[p], h.v[i] = h.v[i], h.v[p]
		i = p
	}
}

func (h *heap) pop() int {
	top := h.v[0]
	last := len(h.k) - 1
	lk, lv := h.k[last], h.v[last]
	h.k, h.v = h.k[:last], h.v[:last]
	if len(h.k) > 0 {
		h.k[0], h.v[0] = lk, lv
		i := 0
		for {
			l := 2*i + 1
			r := l + 1
			m := i
			if l < len(h.k) && h.k[l] < h.k[m] {
				m = l
			}
			if r < len(h.k) && h.k[r] < h.k[m] {
				m = r
			}
			if m == i {
				break
			}
			h.k[m], h.k[i] = h.k[i], h.k[m]
			h.v[m], h.v[i] = h.v[i], h.v[m]
			i = m
		}
	}
	return top
}

func (h *heap) size() int { return len(h.k) }

// Ctx is what the planner knows: broken and slow edges, report confidence, and whether broken edges are impassable.
type Ctx struct {
	Blocked map[int]bool
	Slow    map[int]bool
	Conf    float64
	Hard    bool
}

func NewCtx() Ctx { return Ctx{Blocked: map[int]bool{}, Slow: map[int]bool{}, Conf: 1} }

func (o Ctx) believed(e *Edge) float64 {
	if o.Blocked[e.I] {
		if o.Hard {
			return math.Inf(1)
		}
		return e.Min * (1 + o.Conf*hard)
	}
	if o.Slow[e.I] {
		return e.Min * (1 + o.Conf*slowK)
	}
	return e.Min
}

// DistFrom is travel time (minutes) from one node to every other.
func (n *Net) DistFrom(src int, o Ctx) []float64 {
	dist := make([]float64, len(n.Lat))
	for i := range dist {
		dist[i] = math.Inf(1)
	}
	h := &heap{}
	dist[src] = 0
	h.push(0, src)
	for h.size() > 0 {
		u := h.pop()
		for _, e := range n.Adj[u] {
			w := o.believed(e)
			if math.IsInf(w, 1) {
				continue
			}
			v := e.B
			if e.A != u {
				v = e.A
			}
			if nd := dist[u] + w; nd < dist[v] {
				dist[v] = nd
				h.push(nd, v)
			}
		}
	}
	return dist
}

// Tree is the full shortest-path tree under any cost function (evaluation sweep).
type Tree struct {
	Dist []float64
	Prev []int
	Via  []int
}

func (n *Net) TreeFrom(src int, cost func(*Edge) float64) Tree {
	N := len(n.Lat)
	t := Tree{Dist: make([]float64, N), Prev: make([]int, N), Via: make([]int, N)}
	for i := 0; i < N; i++ {
		t.Dist[i], t.Prev[i], t.Via[i] = math.Inf(1), -1, -1
	}
	h := &heap{}
	t.Dist[src] = 0
	h.push(0, src)
	for h.size() > 0 {
		u := h.pop()
		for _, e := range n.Adj[u] {
			w := cost(e)
			if math.IsInf(w, 1) {
				continue
			}
			v := e.B
			if e.A != u {
				v = e.A
			}
			if nd := t.Dist[u] + w; nd < t.Dist[v] {
				t.Dist[v], t.Prev[v], t.Via[v] = nd, u, e.I
				h.push(nd, v)
			}
		}
	}
	return t
}

type Step struct {
	Name  string  `json:"name"`
	Km    float64 `json:"km"`
	Min   float64 `json:"min"`
	Edges []int   `json:"edges"`
	Slow  bool    `json:"slow"`
	Cut   bool    `json:"cut"`
}

type Route struct {
	Nodes   []int        `json:"nodes"`
	Edges   []int        `json:"edges"`
	Km      float64      `json:"km"`
	Min     float64      `json:"min"`
	Crosses []int        `json:"crosses"`
	Steps   []Step       `json:"steps"`
	Coords  [][2]float64 `json:"coords"` // [lat, lng], as the frontend consumes them
}

// Shortest is A* with a straight-line heuristic. It returns nil only if no road joins a and b.
func (n *Net) Shortest(a, b int, o Ctx) *Route {
	N := len(n.Lat)
	g := make([]float64, N)
	prev := make([]int, N)
	via := make([]int, N)
	done := make([]bool, N)
	for i := 0; i < N; i++ {
		g[i], prev[i], via[i] = math.Inf(1), -1, -1
	}
	est := func(i int) float64 { return Hav(n.Lat[i], n.Lng[i], n.Lat[b], n.Lng[b]) }
	h := &heap{}
	g[a] = 0
	h.push(est(a), a)
	for h.size() > 0 {
		u := h.pop()
		if done[u] {
			continue
		}
		done[u] = true
		if u == b {
			break
		}
		for _, e := range n.Adj[u] {
			w := o.believed(e)
			if math.IsInf(w, 1) {
				continue
			}
			v := e.B
			if e.A != u {
				v = e.A
			}
			if nd := g[u] + w; nd < g[v] {
				g[v], prev[v], via[v] = nd, u, e.I
				h.push(nd+est(v), v)
			}
		}
	}
	if a != b && prev[b] < 0 {
		return nil
	}
	nodes := []int{b}
	var es []int
	for v := b; v != a; v = prev[v] {
		es = append([]int{via[v]}, es...)
		nodes = append([]int{prev[v]}, nodes...)
	}
	r := &Route{Nodes: nodes, Edges: es, Crosses: []int{}, Steps: []Step{}}
	for _, ei := range es {
		e := n.Edges[ei]
		slow, cut := o.Slow[ei], o.Blocked[ei]
		m := e.Min
		if slow {
			m = e.Min * TrueSlow
		}
		r.Km += e.Km
		r.Min += m
		if cut {
			r.Crosses = append(r.Crosses, ei)
		}
		if k := len(r.Steps); k > 0 && r.Steps[k-1].Name == e.Name {
			s := &r.Steps[k-1]
			s.Km += e.Km
			s.Min += m
			s.Edges = append(s.Edges, ei)
			s.Slow = s.Slow || slow
			s.Cut = s.Cut || cut
		} else {
			r.Steps = append(r.Steps, Step{Name: e.Name, Km: e.Km, Min: m, Edges: []int{ei}, Slow: slow, Cut: cut})
		}
	}
	for _, i := range nodes {
		r.Coords = append(r.Coords, [2]float64{n.Lat[i], n.Lng[i]})
	}
	return r
}

// Resolve maps active closure ids and reported breaks (lat/lng points) onto this network.
func (n *Net) Resolve(active []string, kinds map[string]string, breaks []Point) (blocked, slow map[int]bool) {
	blocked, slow = map[int]bool{}, map[int]bool{}
	for _, id := range active {
		for _, e := range n.CID[id] {
			if kinds[id] == "closed" {
				blocked[e] = true
			} else {
				slow[e] = true
			}
		}
	}
	for _, b := range breaks {
		for _, e := range n.EdgesNear(b.Lat, b.Lng, 0.03, false) {
			blocked[e] = true
		}
	}
	return
}

type Point struct {
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Name string  `json:"name,omitempty"`
}
