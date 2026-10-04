package domain

import "testing"

func TestNewAlertDefaultsAndCAP(t *testing.T) {
	a, err := NewAlert("Rispana bank", "", "", "", "Controller", "manual", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Severity != "Severe" || a.TextEN != DefaultEN || a.TextHI != DefaultHI {
		t.Fatalf("defaults: %+v", a)
	}
	infos := a.CAP["info"].([]map[string]any)
	if len(infos) != 2 || infos[0]["language"] != "en-IN" || infos[1]["language"] != "hi-IN" {
		t.Fatalf("cap info: %v", infos)
	}
	if _, err := NewAlert("", "Severe", "x", "", "c", "m", ""); err == nil {
		t.Fatal("area is required")
	}
	if _, err := NewAlert("x", "Catastrophic", "x", "", "c", "m", ""); err == nil {
		t.Fatal("bad severity must be rejected")
	}
}

func TestInfraTextHasHindi(t *testing.T) {
	for _, typ := range []string{"POWER", "TELECOM", "WATER", "OTHER"} {
		en, hi := InfraText(typ, "Dehradun")
		if en == "" || hi == "" {
			t.Fatalf("%s: empty text", typ)
		}
	}
}
