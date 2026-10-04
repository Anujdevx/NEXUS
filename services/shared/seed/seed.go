// Package seed loads the JSON exported by tools/seed/export-data.mjs from the frontend's
// data module (the single source of truth) and ports the frontend's deterministic
// simulated seeding (store.js seedHosp / seedStock) so backend and offline UI agree.
package seed

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// Load reads <dir>/<name>.json into v.
func Load(dir, name string, v any) error {
	b, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if err != nil {
		return fmt.Errorf("seed %s: %w", name, err)
	}
	return json.Unmarshal(b, v)
}

// MustLoad is Load for required files; it panics with a clear message (services recover at boot).
func MustLoad(dir, name string, v any) {
	if err := Load(dir, name, v); err != nil {
		panic(err)
	}
}

type Hospital struct {
	ID    string  `json:"id"`
	N     string  `json:"n"`
	Own   string  `json:"own"`
	Lvl   string  `json:"lvl"`
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	Loc   string  `json:"loc"`
	Ph    string  `json:"ph"`
	Beds  *int    `json:"beds"`
	Node  string  `json:"node"`
	BSrc  string  `json:"bsrc,omitempty"`
	BNote string  `json:"bnote,omitempty"`
}

type Pharmacy struct {
	ID  string  `json:"id"`
	N   string  `json:"n"`
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
	Loc string  `json:"loc"`
	Hrs string  `json:"hrs"`
}

type Medicine struct {
	ID   string `json:"id"`
	N    string `json:"n"`
	Use  string `json:"use"`
	Unit string `json:"unit"`
	Base int    `json:"base"`
}

type Shelter struct {
	ID   string  `json:"id"`
	N    string  `json:"n"`
	Node string  `json:"node"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Cap  int     `json:"cap"`
	Occ  int     `json:"occ"`
	Doc  string  `json:"doc,omitempty"`
	S    string  `json:"s,omitempty"`
}

type Unit struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Node   string `json:"node"`
	Status string `json:"status"`
	Task   string `json:"task"`
}

type Closure struct {
	N    string    `json:"n"`
	Kind string    `json:"kind"` // closed | slow
	Note string    `json:"note"`
	S    string    `json:"s"`
	At   []float64 `json:"at"`
}

type Flood struct {
	ID   string  `json:"id"`
	N    string  `json:"n"`
	R    string  `json:"r"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Note string  `json:"note"`
	Node string  `json:"node"`
}

type Incident struct {
	T     string  `json:"t"`
	N     string  `json:"n"`
	Place string  `json:"place"`
	Sev   string  `json:"sev"`
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	D     string  `json:"d"`
	Node  string  `json:"node"`
}

type Scenario struct {
	Label    string     `json:"label"`
	Short    string     `json:"short"`
	Sum      string     `json:"sum"`
	Closures []string   `json:"closures"`
	Inc      []Incident `json:"inc"`
}

// Node is [lat, lng, name] in NODES.json; Edge is [from, to, road, class, closureId?] in EDGES.json.
type Node = []any
type Edge = []any

type HospState struct {
	Cap     int  `json:"cap"`
	Free    int  `json:"free"`
	Inbound int  `json:"inbound"`
	Divert  bool `json:"divert"`
}

// SeedHosp ports store.js seedHosp: free beds from SIMCAP, deterministic by list position.
func SeedHosp(hs []Hospital, simcap map[string]int) map[string]HospState {
	o := map[string]HospState{}
	for i, h := range hs {
		cap := simcap[h.Lvl]
		free := int(math.Round(float64(cap) * (0.18 + float64((i*37)%50)/100)))
		if free < 1 {
			free = 1
		}
		o[h.ID] = HospState{Cap: cap, Free: free}
	}
	return o
}

// SeedStock ports store.js seedStock: a deterministic sine hash, with about one item in five out of stock.
func SeedStock(ps []Pharmacy, ms []Medicine) map[string]map[string]int {
	o := map[string]map[string]int{}
	for i, p := range ps {
		row := map[string]int{}
		for j, m := range ms {
			x := math.Sin(float64(i+1)*12.9898+float64(j+1)*78.233) * 43758.5453
			h := math.Mod(math.Abs(x), 1)
			if h < 0.2 {
				row[m.ID] = 0
			} else {
				row[m.ID] = int(math.Round(float64(m.Base) * (0.15 + h*1.2)))
			}
		}
		o[p.ID] = row
	}
	return o
}
