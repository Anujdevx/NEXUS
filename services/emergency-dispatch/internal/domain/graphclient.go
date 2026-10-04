package domain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Graph is the client for graph-engine's drive-time and route endpoints.
type Graph struct {
	URL string
	Key string
	HC  *http.Client
}

func NewGraph(url, key string) *Graph {
	return &Graph{URL: url, Key: key, HC: &http.Client{Timeout: 4 * time.Second}}
}

type Target struct {
	ID  string  `json:"id"`
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func (g *Graph) post(ctx context.Context, path string, body, out any) error {
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.URL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Key", g.Key)
	resp, err := g.HC.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("graph-engine %s: %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Times returns drive minutes from a point to each target; unreachable targets map to nil.
func (g *Graph) Times(ctx context.Context, from Point, targets []Target, hard bool) (map[string]*float64, error) {
	var out struct {
		Times map[string]*float64 `json:"times"`
	}
	err := g.post(ctx, "/api/v1/routes/times", map[string]any{"from": map[string]float64{"lat": from.Lat, "lng": from.Lng}, "targets": targets, "hard": hard}, &out)
	return out.Times, err
}

type RouteSummary struct {
	Km      float64  `json:"km"`
	Min     float64  `json:"min"`
	Blocked []string `json:"blocked"`
}

// Plan asks graph-engine for the route (and has it publish route.computed).
func (g *Graph) Plan(ctx context.Context, from, to Point) (*RouteSummary, error) {
	var out RouteSummary
	err := g.post(ctx, "/api/v1/routes/plan", map[string]any{
		"from": map[string]float64{"lat": from.Lat, "lng": from.Lng}, "to": map[string]float64{"lat": to.Lat, "lng": to.Lng}, "publish": true,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
