package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"nexus/shared/bus"
	"nexus/shared/config"
	"nexus/shared/httpx"
	"nexus/shared/pg"
	"nexus/shared/seed"

	dbus "github.com/nexus/emergency-dispatch/internal/bus"
	"github.com/nexus/emergency-dispatch/internal/dispatch"
	"github.com/nexus/emergency-dispatch/internal/domain"
	api "github.com/nexus/emergency-dispatch/internal/http"
	"github.com/nexus/emergency-dispatch/internal/store"
	"github.com/nexus/emergency-dispatch/migrations"
)

func main() {
	ctx := context.Background()
	app := httpx.New("emergency-dispatch")
	dir := config.SeedDir()

	pool, err := pg.Connect(ctx, config.String("DATABASE_URL", "postgres://nexus:nexus@localhost:5432/dispatch?sslmode=disable"), 90*time.Second, app.Log)
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
	if err := st.Seed(ctx, dir, false); err != nil {
		app.Log.Error("seed", "err", err, "hint", "run scripts/seed.sh")
		os.Exit(1)
	}

	var caps map[string][]string
	var modules []struct {
		K    string         `json:"k"`
		Stub map[string]any `json:"stub"`
	}
	seed.MustLoad(dir, "CAPS", &caps)
	seed.MustLoad(dir, "MODULES", &modules)
	stubs := map[string]map[string]any{}
	for _, m := range modules {
		if m.Stub != nil {
			m.Stub["source"] = "stub"
			m.Stub["note"] = "Tier 3 module: a stub envelope, not live data."
			stubs[m.K] = m.Stub
		}
	}

	b := bus.Connect(ctx, config.RabbitURL(), "emergency-dispatch", app.Log)
	d := &dispatch.Dispatcher{
		Store: st, Graph: domain.NewGraph(config.String("GRAPH_URL", "http://nexus-graph-engine:8080"), config.ServiceKey()),
		Pub: b, Caps: caps, Origin: "emergency-dispatch", Log: app.Log,
	}
	cons := &dbus.Consumer{D: d, Log: app.Log, Seed: func(c context.Context) error { return st.Seed(c, dir, true) }}
	b.Subscribe("q.emergency-dispatch", dbus.Keys, cons.Handle)

	app.Ready("postgres", func(c context.Context) error { return pool.Ping(c) })
	app.Ready("rabbitmq", b.Check)
	(&api.API{D: d, Secret: config.JWTSecret(), Key: config.ServiceKey(), Stubs: stubs}).Register(app)
	_ = json.Marshal

	if err := app.Run(ctx); err != nil {
		app.Log.Error("server", "err", err)
		os.Exit(1)
	}
}
