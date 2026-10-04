package domain

import (
	"math"
	"sort"
)

// Rank is rankOnNet(): multi-constraint hospital assignment on reach, capacity, specialty and inbound load.
// times maps hospital id -> drive minutes (a missing or nil entry means cut off). caps maps level -> needs served.
func Rank(hospitals []Hospital, need string, times map[string]*float64, caps map[string][]string) []RankRow {
	rows := make([]RankRow, 0, len(hospitals))
	for _, h := range hospitals {
		t := times[h.ID]
		reachable := t != nil && !math.IsInf(*t, 1)
		capable := contains(caps[h.Lvl], need)
		free := h.Free
		if h.Divert {
			free = 0
		}
		cost := 0.0
		if reachable {
			cost = *t
		} else {
			cost = 999 + 240
		}
		if !capable {
			cost += 45
		}
		if free <= 0 {
			cost += 90
		}
		if free > 0 && free < 3 {
			cost += 8
		}
		cost += float64(h.Inbound) * 5
		row := RankRow{H: h, Reachable: reachable, Capable: capable, Free: free, Inbound: h.Inbound, Cost: cost}
		if reachable {
			v := *t
			row.Min = &v
		}
		rows = append(rows, row)
	}
	// JS Array.prototype.sort is stable.
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Cost < rows[j].Cost })
	return rows
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// NearestHospital is the straight-line baseline the console compares against.
func NearestHospital(hospitals []Hospital, lat, lng float64) Hospital {
	best, bd := hospitals[0], math.Inf(1)
	for _, h := range hospitals {
		if d := Hav(lat, lng, h.Lat, h.Lng); d < bd {
			best, bd = h, d
		}
	}
	return best
}

func Hav(aLat, aLng, bLat, bLng float64) float64 {
	const rad = math.Pi / 180
	dLat, dLng := (bLat-aLat)*rad, (bLng-aLng)*rad
	s := math.Pow(math.Sin(dLat/2), 2) + math.Cos(aLat*rad)*math.Cos(bLat*rad)*math.Pow(math.Sin(dLng/2), 2)
	return 12742 * math.Asin(math.Sqrt(s))
}

// EstimateMinutes is the fallback when graph-engine is unreachable: straight-line km with a winding
// factor at an average hill speed. It is labelled "haversine" so nobody mistakes it for a road route.
func EstimateMinutes(km float64) float64 { return km * 1.4 / 30 * 60 }
