package domain

import (
	"reflect"
	"testing"
)

var segs = []Seg{
	{Closure: "c_itpark", Class: "a", Kind: "slow", Name: "IT Park"},
	{Closure: "c_rajpur", Class: "a", Kind: "slow", Name: "Rajpur"},
	{Closure: "c_kolhu", Class: "m", Kind: "closed", Name: "Kolhukhet"},
	{Closure: "c_nkc", Class: "h", Kind: "closed", Name: "Tons bridge"}, // closed but not a hill road: never rain-inferred
	{Closure: "", Class: "a"},
}

func TestInferFromRainThresholds(t *testing.T) {
	if got := InferFromRain(79, segs); len(got) != 0 {
		t.Fatalf("79 mm: %v", got)
	}
	if got := InferFromRain(80, segs); !reflect.DeepEqual(got, []string{"c_itpark", "c_rajpur"}) {
		t.Fatalf("80 mm: %v", got)
	}
	if got := InferFromRain(150, segs); !reflect.DeepEqual(got, []string{"c_itpark", "c_kolhu", "c_rajpur"}) {
		t.Fatalf("150 mm: %v", got)
	}
}

func TestSegmentsConfidenceAndRisk(t *testing.T) {
	ss := Segments(80, segs)
	if len(ss) != 3 {
		t.Fatalf("%d segments", len(ss))
	}
	for _, s := range ss {
		if s.Confidence < 0.5 || s.Confidence > 0.95 {
			t.Fatalf("confidence out of range: %+v", s)
		}
	}
	if Risk(10) != "low" || Risk(50) != "moderate" || Risk(90) != "high" || Risk(200) != "severe" {
		t.Fatal("risk bands")
	}
}
