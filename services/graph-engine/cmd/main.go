package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"nexus/shared/bus"
	"nexus/shared/config"
	"nexus/shared/httpx"
	"nexus/shared/seed"

	gbus "github.com/nexus/graph-engine/internal/bus"
	"github.com/nexus/graph-engine/internal/domain"
	api "github.com/nexus/graph-engine/internal/http"
	"github.com/nexus/graph-engine/internal/store"
)

func main() {
	ctx := context.Background()
	app := httpx.New("graph-engine")
	dir := config.SeedDir()

	var nodesRaw map[string][]json.RawMessage
	var edgesRaw [][]json.RawMessage
	var speed, wind map[string]float64
	var closures map[string]seed.Closure
	var flood []seed.Flood
	var hospitals []seed.Hospital
	var shelters []seed.Shelter
	var scenarios map[string]seed.Scenario
	var unitsRaw []struct{ ID, Type, Node string }
	var simcap map[string]int
	var caps map[string][]string
	for name, v := range map[string]any{"UNITS": &unitsRaw, "SIMCAP": &simcap, "CAPS": &caps, "NODES": &nodesRaw, "EDGES": &edgesRaw, "SPEED": &speed, "WIND": &wind, "CLOSURES": &closures, "FLOOD": &flood, "HOSPITALS": &hospitals, "SHELTERS": &shelters, "SCENARIOS": &scenarios} {
		if err := seed.Load(dir, name, v); err != nil {
			app.Log.Error("seed", "err", err, "hint", "run scripts/seed.sh")
			os.Exit(1)
		}
	}
	kinds := map[string]string{}
	cat := map[string]domain.ClosureAt{}
	for id, c := range closures {
		kinds[id] = c.Kind
		cat[id] = domain.ClosureAt{Kind: c.Kind, At: [2]float64{c.At[0], c.At[1]}}
	}
	nodes := map[string]domain.NodePos{}
	names := map[int]string{}
	for id, n := range nodesRaw {
		var lat, lng float64
		var name string
		_ = json.Unmarshal(n[0], &lat)
		_ = json.Unmarshal(n[0], &lat)
		_ = json.Unmarshal(n[1], &lng)
		_ = json.Unmarshal(n[2], &name)
		nodes[id] = domain.NodePos{Lat: lat, Lng: lng, Name: name}
	}

	// Road network: real OpenStreetMap roads when public/roads.json exists (same order the frontend uses), else the schematic.
	net := loadNet(app, config.String("ROADS_FILE", "../../frontend/public/roads.json"), cat, nodesRaw, edgesRaw, speed, wind)
	app.Log.Info("road network ready", "source", net.Source, "nodes", len(net.Lat), "edges", len(net.Edges))
	_ = names

	start := scenarios["s260720"]
	state := domain.NewRoadState(start.Closures)
	planner := &domain.Planner{Net: net, Kinds: kinds, Flood: flood, State: state}
	topo := domain.SeedTopology(hospitals, shelters, nodes)
	app.Log.Info("topology seeded (simulated)", "summary", topo.Describe())

	b := bus.Connect(ctx, config.RabbitURL(), "graph-engine", app.Log)
	neo, err := store.Connect(config.String("NEO4J_URI", "bolt://localhost:7687"), config.String("NEO4J_USER", "neo4j"), config.String("NEO4J_PASSWORD", "password"), app.Log)
	if err != nil {
		app.Log.Error("neo4j driver", "err", err)
		os.Exit(1)
	}
	world := domain.EvalWorld{Caps: caps, SimCap: simcap}
	for _, h := range hospitals {
		world.Hospitals = append(world.Hospitals, domain.EvalHospital{Lat: h.Lat, Lng: h.Lng, Lvl: h.Lvl})
	}
	for _, f := range flood {
		world.Flood = append(world.Flood, domain.EvalFlood{Lat: f.Lat, Lng: f.Lng})
	}
	for _, u := range unitsRaw {
		if n, ok := nodes[u.Node]; ok && u.Type == "Ambulance" {
			world.BaseNodes = append(world.BaseNodes, [2]float64{n.Lat, n.Lng})
		}
	}
	svc := &api.Service{World: world, Net: net, Planner: planner, Topo: topo, Nodes: nodes, Pub: b, Secret: config.JWTSecret(), Key: config.ServiceKey(), Origin: "graph-engine"}
	svc.Persist = func(a []domain.Asset) {
		if !neo.Ready() || len(a) == 0 {
			return
		}
		go func() {
			c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := neo.SaveStatuses(c, a); err != nil {
				app.Log.Warn("neo4j save statuses", "err", err)
			}
		}()
	}
	go bootNeo(ctx, app, neo, topo, net, nodesRaw)

	cons := &gbus.Consumer{Svc: svc, Scenarios: scenarios, Log: app.Log}
	b.Subscribe("q.graph-engine", gbus.Keys, cons.Handle)

	app.Ready("road-graph", func(context.Context) error {
		if len(net.Lat) == 0 {
			return fmt.Errorf("empty road graph")
		}
		return nil
	})
	app.Ready("neo4j", neo.Check)
	app.Ready("rabbitmq", b.Check)
	app.OnStop(func(c context.Context) { neo.Close(c) })
	svc.Register(app)

	if err := app.Run(ctx); err != nil {
		app.Log.Error("server", "err", err)
		os.Exit(1)
	}
}

func loadNet(app *httpx.App, path string, cat map[string]domain.ClosureAt, nodesRaw map[string][]json.RawMessage, edgesRaw [][]json.RawMessage, speed, wind map[string]float64) *domain.Net {
	if b, err := os.ReadFile(path); err == nil {
		var f struct {
			Ways []domain.Way `json:"ways"`
		}
		if err := json.Unmarshal(b, &f); err == nil && len(f.Ways) > 50 {
			return domain.Build(f.Ways, "osm-file", cat, 1)
		}
		app.Log.Warn("roads file unusable, using the schematic network", "path", path)
	} else {
		app.Log.Info("no roads file, using the schematic network", "path", path)
	}
	return domain.Build(domain.BuiltinWays(nodesRaw, edgesRaw, speed, wind), "builtin", cat, 1)
}

// bootNeo connects to Neo4j in the background, then seeds it on first boot or loads the persisted topology.
func bootNeo(ctx context.Context, app *httpx.App, neo *store.Neo, topo *domain.Topology, net *domain.Net, nodesRaw map[string][]json.RawMessage) {
	if err := neo.WaitAndInit(ctx); err != nil {
		app.Log.Error("neo4j init", "err", err)
		return
	}
	c, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	n, err := neo.AssetCount(c)
	if err != nil {
		app.Log.Error("neo4j count", "err", err)
		return
	}
	if net.Source == "builtin" {
		if err := neo.SaveRoads(c, net, nil); err != nil {
			app.Log.Warn("neo4j save roads", "err", err)
		}
	}
	if n == 0 {
		if err := neo.SaveTopology(c, topo.List(), topo.DepList()); err != nil {
			app.Log.Error("neo4j seed topology", "err", err)
			return
		}
		app.Log.Info("neo4j seeded with the simulated asset graph")
		return
	}
	assets, deps, err := neo.LoadTopology(c)
	if err != nil || len(assets) == 0 {
		app.Log.Warn("neo4j load topology failed, keeping the in-memory seed", "err", err)
		return
	}
	loaded := domain.NewTopology()
	for _, a := range assets {
		loaded.Put(a)
	}
	for _, d := range deps {
		loaded.AddDep(d)
	}
	topo.Replace(loaded)
	app.Log.Info("topology loaded from neo4j", "assets", len(assets), "deps", len(deps))
}
