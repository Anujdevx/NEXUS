package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"nexus/shared/bus"
	"nexus/shared/config"
	"nexus/shared/httpx"
	"nexus/shared/pg"
	"nexus/shared/seed"

	tbus "github.com/nexus/traffic-control/internal/bus"
	api "github.com/nexus/traffic-control/internal/http"
	"github.com/nexus/traffic-control/internal/store"
	"github.com/nexus/traffic-control/migrations"
)

const defaultScenario = "s260720" // the console's opening scenario

func main() {
	ctx := context.Background()
	app := httpx.New("traffic-control")
	pool, err := pg.Connect(ctx, config.String("DATABASE_URL", "postgres://nexus:nexus@localhost:5432/traffic?sslmode=disable"), 90*time.Second, app.Log)
	if err != nil {
		app.Log.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pg.Migrate(ctx, pool, migrations.FS, ".", app.Log); err != nil {
		app.Log.Error("migrate", "err", err)
		os.Exit(1)
	}
	st := &store.Store{DB: pool}
	if empty, err := st.Empty(ctx); err == nil && empty {
		if err := st.Seed(ctx, config.SeedDir(), defaultScenario); err != nil {
			app.Log.Error("seed", "err", err, "hint", "run scripts/seed.sh")
			os.Exit(1)
		}
		app.Log.Info("closures seeded", "scenario", defaultScenario)
	}
	var scenarios map[string]seed.Scenario
	seed.MustLoad(config.SeedDir(), "SCENARIOS", &scenarios)

	b := bus.Connect(ctx, config.RabbitURL(), "traffic-control", app.Log)
	b.Subscribe("q.traffic-control", tbus.Keys, (&tbus.Consumer{Store: st, Scenarios: scenarios}).Handle)
	app.Ready("postgres", func(c context.Context) error { return pool.Ping(c) })
	app.Ready("rabbitmq", b.Check)
	(&api.API{Store: st, Pub: b, Secret: config.JWTSecret(), Key: config.ServiceKey(), GraphURL: config.String("GRAPH_URL", "http://nexus-graph-engine:8080"), HC: &http.Client{Timeout: 4 * time.Second}}).Register(app)

	if err := app.Run(ctx); err != nil {
		app.Log.Error("server", "err", err)
		os.Exit(1)
	}
}
