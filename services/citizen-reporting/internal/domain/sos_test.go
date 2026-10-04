package domain

import "testing"

func TestNormalizeFillsDefaultsAndRejectsBadInput(t *testing.T) {
	s := SOS{Hazard: "Flood", Lat: 30.38, Lng: 78.13, Injured: "maybe"}
	if err := s.Normalize(); err != nil {
		t.Fatal(err)
	}
	if s.People != 1 || s.Injured != "No" || s.Via != "direct" || !idRe.MatchString(s.ID) {
		t.Fatalf("bad defaults: %+v", s)
	}
	bad := SOS{Hazard: "Flood"}
	if bad.Normalize() == nil {
		t.Fatal("missing location must be rejected")
	}
}

func TestIncidentShape(t *testing.T) {
	s := SOS{ID: "SOS-12345", Hazard: "Trapped", People: 4, Injured: "Yes", PlaceName: "Sahastradhara", Lat: 30.38, Lng: 78.13, Via: "direct"}
	i := IncidentFromSOS(s)
	if i.Sev != "High" || i.N != "SOS: Trapped, 4 people" || !i.Sim || i.Status != "Open" {
		t.Fatalf("%+v", i)
	}
}
