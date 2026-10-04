package domain

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

type fixPick struct {
	Nodes   []int   `json:"nodes"`
	Edges   []int   `json:"edges"`
	Km      float64 `json:"km"`
	Min     float64 `json:"min"`
	Crosses []int   `json:"crosses"`
	Steps   []struct {
		Name string  `json:"name"`
		Km   float64 `json:"km"`
		Min  float64 `json:"min"`
		Slow bool    `json:"slow"`
		Cut  bool    `json:"cut"`
	} `json:"steps"`
}

type fixture struct {
	Nodes, Edges int
	Cases        []struct {
		From, To string
		Closures []string
		Breaks   []Point
		Conf     float64
		A, B     int
		Soft     *fixPick
		Hard     *fixPick
		Blocked  []int
		Slow     []int
	}
	Lookups []struct {
		La, Ln   float64
		Node     int
		Edge     *int
		EdgeDist float64
		Near     int
	}
	DistFromCT []*float64
	CID        map[string][]int
}

func seedDir() string {
	if d := os.Getenv("SEED_DIR"); d != "" {
		return d
	}
	return "../../../../tools/seed/out"
}

func loadBuiltin(t *testing.T) (*Net, map[string]ClosureAt, map[string]string) {
	rd := func(name string, v any) {
		b, err := os.ReadFile(seedDir() + "/" + name + ".json")
		if err != nil {
			t.Skip("run `node tools/seed/export-data.mjs` first: ", err)
		}
		if err := json.Unmarshal(b, v); err != nil {
			t.Fatal(err)
		}
	}
	var nodes map[string][]json.RawMessage
	var edges [][]json.RawMessage
	var speed, wind map[string]float64
	var cl map[string]struct {
		Kind string
		At   [2]float64
	}
	rd("NODES", &nodes)
	rd("EDGES", &edges)
	rd("SPEED", &speed)
	rd("WIND", &wind)
	rd("CLOSURES", &cl)
	kinds := map[string]string{}
	ca := map[string]ClosureAt{}
	for id, c := range cl {
		kinds[id] = c.Kind
		ca[id] = ClosureAt{c.Kind, c.At}
	}
	return Build(BuiltinWays(nodes, edges, speed, wind), "builtin", ca, 1), ca, kinds
}

func near(a, b float64) bool { return math.Abs(a-b) <= 1e-9*math.Max(1, math.Abs(b)) }

// Compares 10+ fixed A->B pairs against the output of the frontend's roadnet.js.
func TestRoutesMatchFrontendEngine(t *testing.T) {
	n, _, kinds := loadBuiltin(t)
	b, err := os.ReadFile("../../testdata/routes.json")
	if err != nil {
		t.Fatal(err)
	}
	var fx fixture
	if err := json.Unmarshal(b, &fx); err != nil {
		t.Fatal(err)
	}
	if len(n.Lat) != fx.Nodes || len(n.Edges) != fx.Edges {
		t.Fatalf("graph size: got %d/%d want %d/%d", len(n.Lat), len(n.Edges), fx.Nodes, fx.Edges)
	}
	if len(fx.Cases) < 10 {
		t.Fatalf("need at least 10 cases, have %d", len(fx.Cases))
	}
	for _, c := range fx.Cases {
		name := c.From + "->" + c.To
		blocked, slow := n.Resolve(c.Closures, kinds, c.Breaks)
		if len(blocked) != len(c.Blocked) || len(slow) != len(c.Slow) {
			t.Errorf("%s: blocked/slow sets differ: got %d/%d want %d/%d", name, len(blocked), len(slow), len(c.Blocked), len(c.Slow))
		}
		ctx := Ctx{Blocked: blocked, Slow: slow, Conf: c.Conf}
		for label, want := range map[string]*fixPick{"soft": c.Soft, "hard": c.Hard} {
			cx := ctx
			cx.Hard = label == "hard"
			got := n.Shortest(c.A, c.B, cx)
			if want == nil {
				if got != nil {
					t.Errorf("%s/%s: expected no route", name, label)
				}
				continue
			}
			if got == nil {
				t.Errorf("%s/%s: no route, want %d edges", name, label, len(want.Edges))
				continue
			}
			if !equalInts(got.Nodes, want.Nodes) || !equalInts(got.Edges, want.Edges) {
				t.Errorf("%s/%s: path differs\n got %v\nwant %v", name, label, got.Nodes, want.Nodes)
			}
			if !near(got.Km, want.Km) || !near(got.Min, want.Min) {
				t.Errorf("%s/%s: km/min got %.6f/%.6f want %.6f/%.6f", name, label, got.Km, got.Min, want.Km, want.Min)
			}
			if len(got.Steps) != len(want.Steps) || !equalInts(got.Crosses, want.Crosses) {
				t.Errorf("%s/%s: steps/crosses differ", name, label)
			}
		}
	}
	for _, l := range fx.Lookups {
		if got := n.NearestNode(l.La, l.Ln); got != l.Node {
			t.Errorf("nearestNode(%v,%v)=%d want %d", l.La, l.Ln, got, l.Node)
		}
		e, d := n.NearestEdge(l.La, l.Ln)
		if e == nil || (l.Edge != nil && e.I != *l.Edge) || !near(d, l.EdgeDist) {
			t.Errorf("nearestEdge(%v,%v) differs", l.La, l.Ln)
		}
		if got := len(n.EdgesNear(l.La, l.Ln, 0.5, false)); got != l.Near {
			t.Errorf("edgesNear(%v,%v)=%d want %d", l.La, l.Ln, got, l.Near)
		}
	}
	d := n.DistFrom(n.NearestNode(30.3245, 78.0418), Ctx{Blocked: map[int]bool{}, Slow: map[int]bool{}, Conf: 1})
	for i, w := range fx.DistFromCT {
		if w == nil {
			if !math.IsInf(d[i], 1) {
				t.Errorf("distFrom[%d] should be unreachable", i)
			}
			continue
		}
		if !near(d[i], *w) {
			t.Errorf("distFrom[%d]=%v want %v", i, d[i], *w)
		}
	}
	for id, want := range fx.CID {
		if !equalInts(n.CID[id], want) {
			t.Errorf("closure %s edges %v want %v", id, n.CID[id], want)
		}
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
