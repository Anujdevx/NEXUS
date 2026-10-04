package domain

import "testing"

func chain() *Topology {
	t := NewTopology()
	t.Put(Asset{ID: "sub", Type: "POWER", Load: 60, Threshold: 100, GraceMs: 1000})
	t.Put(Asset{ID: "pump", Type: "WATER", Load: 45, Threshold: 100})
	t.Put(Asset{ID: "hosp", Type: "HOSPITAL", Load: 40, Threshold: 100})
	t.Put(Asset{ID: "shelter", Type: "SHELTER", Load: 30, Threshold: 100})
	t.AddDep(Dep{Dependent: "pump", Provider: "sub", Criticality: 0.8})  // 45+80 > 100 -> failed
	t.AddDep(Dep{Dependent: "hosp", Provider: "pump", Criticality: 0.4}) // 40+40 -> degraded
	t.AddDep(Dep{Dependent: "hosp", Provider: "sub", Criticality: 0.3})  // 80+30 -> failed
	t.AddDep(Dep{Dependent: "shelter", Provider: "pump", Criticality: 0.3})
	return t
}

func TestCascadeWavesAndStatuses(t *testing.T) {
	tp := chain()
	res, ok := tp.Fail("sub", true)
	if !ok {
		t.Fatal("unknown asset")
	}
	// wave 1: pump failed + hosp degraded by the substation (40+30=70) ; wave 2: hosp via pump (70+40=110 failed), shelter degraded
	if len(res.Waves) != 2 {
		t.Fatalf("waves=%v", res.Waves)
	}
	pump, _ := tp.Get("pump")
	hosp, _ := tp.Get("hosp")
	shel, _ := tp.Get("shelter")
	if pump.Status != Failed || hosp.Status != Failed || shel.Status != Degraded {
		t.Fatalf("statuses pump=%s hosp=%s shelter=%s", pump.Status, hosp.Status, shel.Status)
	}
}

func TestBlastRadiusIsDryRun(t *testing.T) {
	tp := chain()
	res, _ := tp.Fail("sub", false)
	if len(res.Failed) < 2 {
		t.Fatalf("expected failures, got %v", res.Failed)
	}
	if a, _ := tp.Get("pump"); a.Status != Active {
		t.Fatal("dry run mutated the topology")
	}
}

func TestTelemetryGracePeriod(t *testing.T) {
	tp := chain()
	t0 := timeAt(0)
	if trip, _ := tp.Telemetry("sub", 115, t0); trip {
		t.Fatal("tripped before the grace period")
	}
	if trip, _ := tp.Telemetry("sub", 116, t0.Add(500*msec)); trip {
		t.Fatal("tripped at 500ms")
	}
	if trip, _ := tp.Telemetry("sub", 117, t0.Add(1100*msec)); !trip {
		t.Fatal("should trip after the grace period")
	}
	tp2 := chain()
	tp2.Telemetry("sub", 115, t0)
	tp2.Telemetry("sub", 80, t0.Add(900*msec)) // recovers
	if trip, _ := tp2.Telemetry("sub", 115, t0.Add(1500*msec)); trip {
		t.Fatal("grace timer should have reset")
	}
}
