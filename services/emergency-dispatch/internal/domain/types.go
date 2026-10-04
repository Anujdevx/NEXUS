// Package domain holds emergency-dispatch's rules: hospital allocation (a port of rankOnNet in
// frontend/src/state/store.js), ambulance choice, and the dispatch orchestrator.
package domain

import "nexus/shared/seed"

// Hospital is a registry record (real: name, place, phone) plus simulated capacity state.
type Hospital struct {
	seed.Hospital
	Cap     int    `json:"cap"`
	Free    int    `json:"free"`
	Inbound int    `json:"inbound"`
	Divert  bool   `json:"divert"`
	Source  string `json:"source"` // "nic" (registry) for identity; capacity is simulated
	CapSrc  string `json:"capacity_source"`
}

type Unit struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"`
	Node   string  `json:"node"`
	Status string  `json:"status"`
	Task   string  `json:"task"`
	Step   *string `json:"step,omitempty"`
	Lat    float64 `json:"lat"`
	Lng    float64 `json:"lng"`
}

type Shelter struct {
	seed.Shelter
	Open bool `json:"open"`
}

type Pharmacy struct {
	seed.Pharmacy
	Stock map[string]int `json:"stock,omitempty"`
}

// RankRow mirrors a row of rankOnNet(): h is the hospital, min the drive time (null when cut off).
type RankRow struct {
	H         Hospital `json:"h"`
	Min       *float64 `json:"min"`
	Reachable bool     `json:"reachable"`
	Capable   bool     `json:"capable"`
	Free      int      `json:"free"`
	Inbound   int      `json:"inbound"`
	Cost      float64  `json:"cost"`
}

type Point struct {
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Name string  `json:"name,omitempty"`
}
