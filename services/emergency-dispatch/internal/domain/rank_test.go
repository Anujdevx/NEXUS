package domain

import (
	"testing"

	"nexus/shared/seed"
)

func f(v float64) *float64 { return &v }

var caps = map[string][]string{"Tertiary": {"Trauma", "Cardiac", "Maternity", "General"}, "Secondary": {"Trauma", "Maternity", "General"}, "CHC": {"Maternity", "General"}, "PHC": {"General"}}

func hosp(id, lvl string, free, inbound int, divert bool) Hospital {
	return Hospital{Hospital: seed.Hospital{ID: id, Lvl: lvl}, Free: free, Inbound: inbound, Divert: divert}
}

func TestRankPrefersCapableOpenAndNearby(t *testing.T) {
	hs := []Hospital{hosp("near-phc", "PHC", 3, 0, false), hosp("mid-sec", "Secondary", 10, 0, false), hosp("far-ter", "Tertiary", 30, 0, false)}
	times := map[string]*float64{"near-phc": f(5), "mid-sec": f(20), "far-ter": f(40)}
	rows := Rank(hs, "Trauma", times, caps)
	if rows[0].H.ID != "mid-sec" {
		t.Fatalf("trauma should go to the secondary, got %s", rows[0].H.ID)
	}
	if rows[0].Cost != 20 {
		t.Fatalf("cost=%v", rows[0].Cost)
	}
}

func TestRankPenalisesDivertInboundAndCutOff(t *testing.T) {
	hs := []Hospital{hosp("a", "Tertiary", 20, 0, true), hosp("b", "Tertiary", 20, 4, false), hosp("c", "Tertiary", 20, 0, false)}
	times := map[string]*float64{"a": f(5), "b": f(10), "c": nil}
	rows := Rank(hs, "General", times, caps)
	// a: 5+90 = 95, b: 10+20 = 30, c: 999+240
	if rows[0].H.ID != "b" || rows[1].H.ID != "a" || rows[2].H.ID != "c" {
		t.Fatalf("order %s %s %s", rows[0].H.ID, rows[1].H.ID, rows[2].H.ID)
	}
	if rows[2].Reachable || rows[2].Min != nil {
		t.Fatal("cut-off hospital must not be reachable")
	}
	if rows[1].Free != 0 {
		t.Fatal("diverting hospital reports zero free beds")
	}
}

func TestRankLowFreeBedPenalty(t *testing.T) {
	rows := Rank([]Hospital{hosp("a", "Tertiary", 2, 0, false)}, "General", map[string]*float64{"a": f(10)}, caps)
	if rows[0].Cost != 18 {
		t.Fatalf("cost=%v want 18", rows[0].Cost)
	}
}
