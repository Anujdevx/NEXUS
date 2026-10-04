package main

import (
	"context"
	"os"

	"nexus/shared/bus"
	"nexus/shared/config"
	"nexus/shared/httpx"
	"nexus/shared/seed"

	pbus "github.com/nexus/predictive-analytics/internal/bus"
	"github.com/nexus/predictive-analytics/internal/domain"
	api "github.com/nexus/predictive-analytics/internal/http"
)

func main() {
	ctx := context.Background()
	app := httpx.New("predictive-analytics")
	dir := config.SeedDir()

	var edges [][]any
	var closures map[string]seed.Closure
	seed.MustLoad(dir, "EDGES", &edges)
	seed.MustLoad(dir, "CLOSURES", &closures)
	var segs []domain.Seg
	for _, e := range edges {
		if len(e) < 5 {
			continue
		}
		id, _ := e[4].(string)
		cl, ok := closures[id]
		if id == "" || !ok {
			continue
		}
		road, _ := e[2].(string)
		class, _ := e[3].(string)
		segs = append(segs, domain.Seg{Closure: id, Road: road, Class: class, Kind: cl.Kind, Name: cl.N})
	}
	app.Log.Info("weak segments loaded", "segments", len(segs))

	b := bus.Connect(ctx, config.RabbitURL(), "predictive-analytics", app.Log)
	cache := api.NewCache(config.RedisURL())
	a := api.New(segs, b, config.JWTSecret(), config.ServiceKey(), config.String("GRAPH_URL", "http://nexus-graph-engine:8080"), config.String("TRAFFIC_URL", "http://nexus-traffic-control:8080"), cache)
	b.Subscribe("q.predictive-analytics", pbus.Keys, (&pbus.Consumer{API: a}).Handle)
	app.Ready("rabbitmq", b.Check)
	app.Ready("redis", cache.Ping)
	a.Register(app)
	if err := app.Run(ctx); err != nil {
		app.Log.Error("server", "err", err)
		os.Exit(1)
	}
}
