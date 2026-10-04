package main

import (
	"context"
	"os"
	"time"

	"nexus/shared/bus"
	"nexus/shared/config"
	"nexus/shared/httpx"
	"nexus/shared/pg"

	ebus "github.com/nexus/environmental-monitor/internal/bus"
	api "github.com/nexus/environmental-monitor/internal/http"
	"github.com/nexus/environmental-monitor/internal/store"
	"github.com/nexus/environmental-monitor/migrations"
)

func main() {
	ctx := context.Background()
	app := httpx.New("environmental-monitor")
	pool, err := pg.Connect(ctx, config.String("DATABASE_URL", "postgres://nexus:nexus@localhost:5432/environment?sslmode=disable"), 90*time.Second, app.Log)
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
		if err := st.Seed(ctx, config.SeedDir()); err != nil {
			app.Log.Error("seed", "err", err, "hint", "run scripts/seed.sh")
			os.Exit(1)
		}
		app.Log.Info("environment seeded from the frontend datasets")
	}
	b := bus.Connect(ctx, config.RabbitURL(), "environmental-monitor", app.Log)
	a := &api.API{Store: st, Pub: b, Secret: config.JWTSecret(), Key: config.ServiceKey()}
	b.Subscribe("q.environmental-monitor", ebus.Keys, (&ebus.Consumer{Store: st, API: a, Pub: b}).Handle)
	app.Ready("postgres", func(c context.Context) error { return pool.Ping(c) })
	app.Ready("rabbitmq", b.Check)
	a.Register(app)
	if err := app.Run(ctx); err != nil {
		app.Log.Error("server", "err", err)
		os.Exit(1)
	}
}
