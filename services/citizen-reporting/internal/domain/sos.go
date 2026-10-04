// Package domain holds citizen-reporting's rules: shaping an SOS into an incident the console understands.
package domain

import (
	"fmt"
	"math/rand"
	"regexp"
	"time"
)

// SOS is what the citizen device sends (and what the console's sendSos builds).
type SOS struct {
	ID        string  `json:"id"`
	Place     string  `json:"place"`
	PlaceName string  `json:"placeName"`
	Hazard    string  `json:"hazard"`
	People    int     `json:"people"`
	Injured   string  `json:"injured"` // "Yes" | "No"
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Node      string  `json:"node"`
	Via       string  `json:"via"`
	Raised    string  `json:"raised"`
	Client    string  `json:"client"`
}

var idRe = regexp.MustCompile(`^SOS-[A-Za-z0-9]{3,12}$`)

func (s *SOS) Normalize() error {
	if s.Hazard == "" {
		return fmt.Errorf("hazard is required")
	}
	if s.Lat < -90 || s.Lat > 90 || s.Lng < -180 || s.Lng > 180 || (s.Lat == 0 && s.Lng == 0) {
		return fmt.Errorf("lat and lng are required")
	}
	if s.People < 1 {
		s.People = 1
	}
	if s.Injured != "Yes" {
		s.Injured = "No"
	}
	if !idRe.MatchString(s.ID) {
		s.ID = fmt.Sprintf("SOS-%05d", 10000+rand.Intn(89999))
	}
	if s.Via == "" {
		s.Via = "direct"
	}
	if s.PlaceName == "" {
		s.PlaceName = s.Place
	}
	if s.Raised == "" {
		s.Raised = time.Now().UTC().Format(time.RFC3339)
	}
	return nil
}

func (s SOS) Severity() string {
	if s.Injured == "Yes" {
		return "High"
	}
	return "Medium"
}

// Incident is the console's incident shape (see scenarioIncidents / deliverSos in store.js).
type Incident struct {
	ID     string     `json:"id"`
	T      string     `json:"t"`
	N      string     `json:"n"`
	Place  string     `json:"place"`
	Sev    string     `json:"sev"`
	Lat    float64    `json:"lat"`
	Lng    float64    `json:"lng"`
	Node   string     `json:"node"`
	D      string     `json:"d"`
	S      []string   `json:"s"`
	Sim    bool       `json:"sim"`
	Status string     `json:"status"`
	Unit   *string    `json:"unit"`
	When   string     `json:"when"`
	Log    [][]string `json:"log"`
}

func IncidentFromSOS(s SOS) Incident {
	t := time.Now().Format("03:04 PM")
	return Incident{
		ID: s.ID, T: "SOS", N: fmt.Sprintf("SOS: %s, %d people", s.Hazard, s.People), Place: s.PlaceName, Sev: s.Severity(),
		Lat: s.Lat, Lng: s.Lng, Node: s.Node, D: fmt.Sprintf("Raised from a citizen device. Delivered by %s.", s.Via), S: []string{},
		Sim: true, Status: "Open", Unit: nil, When: "just now",
		Log: [][]string{{"Raised on a citizen device", t}, {"Delivered by " + s.Via, t}},
	}
}
