package seed

import (
	"os"
	"testing"
)

func dir() string {
	if d := os.Getenv("SEED_DIR"); d != "" {
		return d
	}
	return "../../../tools/seed/out"
}

// The Go ports must reproduce exactly what the frontend's own functions produced.
func TestSeedPortsMatchFrontend(t *testing.T) {
	if _, err := os.Stat(dir() + "/HOSP_STATE.json"); err != nil {
		t.Skip("run `node tools/seed/export-data.mjs` first")
	}
	var hs []Hospital
	var simcap map[string]int
	var want map[string]HospState
	var ps []Pharmacy
	var ms []Medicine
	var wantStock map[string]map[string]int
	MustLoad(dir(), "HOSPITALS", &hs)
	MustLoad(dir(), "SIMCAP", &simcap)
	MustLoad(dir(), "HOSP_STATE", &want)
	MustLoad(dir(), "PHARMACIES", &ps)
	MustLoad(dir(), "MEDICINES", &ms)
	MustLoad(dir(), "STOCK", &wantStock)
	got := SeedHosp(hs, simcap)
	for id, w := range want {
		if got[id] != w {
			t.Errorf("hospital %s: got %+v want %+v", id, got[id], w)
		}
	}
	gs := SeedStock(ps, ms)
	for pid, row := range wantStock {
		for mid, q := range row {
			if gs[pid][mid] != q {
				t.Errorf("stock %s/%s: got %d want %d", pid, mid, gs[pid][mid], q)
			}
		}
	}
}
