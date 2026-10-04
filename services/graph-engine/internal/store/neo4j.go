// Package store persists the infrastructure graph in Neo4j: (:Junction)-[:ROAD]->(:Junction) for the
// schematic network, and (:Asset)-[:DEPENDS_ON|:NEAR|:LOCATED_IN] for Prāṇadhārā.
// Routing and the cascade run in memory; Neo4j is the durable copy and is loaded on boot.
package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/nexus/graph-engine/internal/domain"
)

type Neo struct {
	drv   neo4j.DriverWithContext
	ready atomic.Bool
	log   *slog.Logger
}

func Connect(uri, user, pass string, log *slog.Logger) (*Neo, error) {
	d, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, pass, ""))
	if err != nil {
		return nil, err
	}
	return &Neo{drv: d, log: log}, nil
}

func (n *Neo) Ready() bool { return n.ready.Load() }

func (n *Neo) Check(ctx context.Context) error {
	if !n.Ready() {
		return errors.New("neo4j not initialised yet")
	}
	return n.drv.VerifyConnectivity(ctx)
}

func (n *Neo) Close(ctx context.Context) { _ = n.drv.Close(ctx) }

func (n *Neo) run(ctx context.Context, cypher string, params map[string]any) (*neo4j.EagerResult, error) {
	return neo4j.ExecuteQuery(ctx, n.drv, cypher, params, neo4j.EagerResultTransformer, neo4j.ExecuteQueryWithDatabase("neo4j"))
}

// WaitAndInit blocks (retrying) until Neo4j answers, creates constraints, then marks the store ready.
func (n *Neo) WaitAndInit(ctx context.Context) error {
	backoff := 2 * time.Second
	for {
		vctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := n.drv.VerifyConnectivity(vctx)
		cancel()
		if err == nil {
			for _, q := range []string{
				`CREATE CONSTRAINT junction_id IF NOT EXISTS FOR (j:Junction) REQUIRE j.id IS UNIQUE`,
				`CREATE CONSTRAINT asset_id IF NOT EXISTS FOR (a:Asset) REQUIRE a.id IS UNIQUE`,
				`CREATE CONSTRAINT region_id IF NOT EXISTS FOR (r:Region) REQUIRE r.id IS UNIQUE`,
			} {
				if _, err := n.run(ctx, q, nil); err != nil {
					return fmt.Errorf("constraint: %w", err)
				}
			}
			n.ready.Store(true)
			return nil
		}
		n.log.Info("waiting for neo4j", "err", err.Error())
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}
		if backoff < 10*time.Second {
			backoff += 2 * time.Second
		}
	}
}

func (n *Neo) AssetCount(ctx context.Context) (int, error) {
	r, err := n.run(ctx, `MATCH (a:Asset) RETURN count(a) AS c`, nil)
	if err != nil || len(r.Records) == 0 {
		return 0, err
	}
	c, _ := r.Records[0].Get("c")
	v, _ := c.(int64)
	return int(v), nil
}

// SaveTopology writes assets, regions, NEAR and DEPENDS_ON in one pass (idempotent MERGE).
func (n *Neo) SaveTopology(ctx context.Context, assets []domain.Asset, deps []domain.Dep) error {
	rows := make([]map[string]any, len(assets))
	for i, a := range assets {
		rows[i] = map[string]any{"id": a.ID, "name": a.Name, "type": a.Type, "lat": a.Lat, "lng": a.Lng, "status": a.Status,
			"load": a.Load, "base": a.BaseLoad, "thr": a.Threshold, "grace": a.GraceMs, "source": a.Source, "ref": a.Ref, "node": a.Node}
	}
	if _, err := n.run(ctx, `MERGE (:Region {id:'dehradun', name:'Dehradun district'})`, nil); err != nil {
		return err
	}
	if _, err := n.run(ctx, `UNWIND $rows AS r
		MERGE (a:Asset {id:r.id})
		SET a.name=r.name, a.type=r.type, a.lat=r.lat, a.lng=r.lng, a.status=r.status, a.load_percentage=r.load,
		    a.base_load=r.base, a.failure_threshold=r.thr, a.grace_period_ms=r.grace, a.source=r.source, a.ref=r.ref, a.node=r.node
		WITH a, r
		MATCH (g:Region {id:'dehradun'}) MERGE (a)-[:LOCATED_IN]->(g)
		WITH a, r
		OPTIONAL MATCH (j:Junction {id:r.node})
		FOREACH (_ IN CASE WHEN j IS NULL THEN [] ELSE [1] END | MERGE (a)-[:NEAR]->(j))`, map[string]any{"rows": rows}); err != nil {
		return err
	}
	drows := make([]map[string]any, len(deps))
	for i, d := range deps {
		drows[i] = map[string]any{"a": d.Dependent, "b": d.Provider, "c": d.Criticality, "g": d.GraceMs}
	}
	_, err := n.run(ctx, `UNWIND $rows AS r
		MATCH (a:Asset {id:r.a}), (b:Asset {id:r.b})
		MERGE (a)-[d:DEPENDS_ON]->(b) SET d.criticality=r.c, d.grace_period_ms=r.g`, map[string]any{"rows": drows})
	return err
}

// LoadTopology reads assets and DEPENDS_ON edges back (used on boot so persisted statuses survive restarts).
func (n *Neo) LoadTopology(ctx context.Context) ([]domain.Asset, []domain.Dep, error) {
	r, err := n.run(ctx, `MATCH (a:Asset) RETURN a ORDER BY a.id`, nil)
	if err != nil {
		return nil, nil, err
	}
	var assets []domain.Asset
	for _, rec := range r.Records {
		v, _ := rec.Get("a")
		node, ok := v.(neo4j.Node)
		if !ok {
			continue
		}
		p := node.Props
		assets = append(assets, domain.Asset{
			ID: str(p["id"]), Name: str(p["name"]), Type: str(p["type"]), Lat: f(p["lat"]), Lng: f(p["lng"]), Status: str(p["status"]),
			Load: f(p["load_percentage"]), BaseLoad: f(p["base_load"]), Threshold: f(p["failure_threshold"]), GraceMs: int(f(p["grace_period_ms"])),
			Source: str(p["source"]), Ref: str(p["ref"]), Node: str(p["node"]), Region: "dehradun",
		})
	}
	dr, err := n.run(ctx, `MATCH (a:Asset)-[d:DEPENDS_ON]->(b:Asset) RETURN a.id AS a, b.id AS b, d.criticality AS c, d.grace_period_ms AS g`, nil)
	if err != nil {
		return nil, nil, err
	}
	var deps []domain.Dep
	for _, rec := range dr.Records {
		a, _ := rec.Get("a")
		b, _ := rec.Get("b")
		c, _ := rec.Get("c")
		g, _ := rec.Get("g")
		deps = append(deps, domain.Dep{Dependent: str(a), Provider: str(b), Criticality: f(c), GraceMs: int(f(g))})
	}
	return assets, deps, nil
}

// SaveStatuses persists status and load for the assets a cascade changed.
func (n *Neo) SaveStatuses(ctx context.Context, assets []domain.Asset) error {
	rows := make([]map[string]any, len(assets))
	for i, a := range assets {
		rows[i] = map[string]any{"id": a.ID, "status": a.Status, "load": a.Load}
	}
	_, err := n.run(ctx, `UNWIND $rows AS r MATCH (a:Asset {id:r.id}) SET a.status=r.status, a.load_percentage=r.load`, map[string]any{"rows": rows})
	return err
}

// SaveRoads stores the schematic road graph (builtin network only: the OSM network has ~170k junctions
// and stays in memory, loaded from public/roads.json).
func (n *Neo) SaveRoads(ctx context.Context, net *domain.Net, names map[int]string) error {
	js := make([]map[string]any, len(net.Lat))
	for i := range net.Lat {
		js[i] = map[string]any{"id": fmt.Sprint(i), "name": names[i], "lat": net.Lat[i], "lng": net.Lng[i]}
	}
	if _, err := n.run(ctx, `UNWIND $rows AS r MERGE (j:Junction {id:r.id}) SET j.name=r.name, j.lat=r.lat, j.lng=r.lng`, map[string]any{"rows": js}); err != nil {
		return err
	}
	es := make([]map[string]any, 0, len(net.Edges))
	for _, e := range net.Edges {
		es = append(es, map[string]any{"id": e.I, "a": fmt.Sprint(e.A), "b": fmt.Sprint(e.B), "name": e.Name, "class": e.HW, "km": e.Km, "speed": e.Km / e.Min * 60})
	}
	_, err := n.run(ctx, `UNWIND $rows AS r
		MATCH (a:Junction {id:r.a}), (b:Junction {id:r.b})
		MERGE (a)-[x:ROAD {id:r.id}]->(b) SET x.name=r.name, x.class=r.class, x.km=r.km, x.speedKmh=r.speed, x.status='OPEN', x.confidence=1.0
		MERGE (b)-[y:ROAD {id:r.id+100000}]->(a) SET y.name=r.name, y.class=r.class, y.km=r.km, y.speedKmh=r.speed, y.status='OPEN', y.confidence=1.0`, map[string]any{"rows": es})
	return err
}

func str(v any) string { s, _ := v.(string); return s }
func f(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int64:
		return float64(x)
	}
	return 0
}
