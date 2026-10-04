package main

import (
	"context"
	"os"
	"time"

	"nexus/shared/bus"
	"nexus/shared/config"
	"nexus/shared/httpx"
	"nexus/shared/pg"
	"nexus/shared/ratelimit"

	cbus "github.com/nexus/citizen-reporting/internal/bus"
	api "github.com/nexus/citizen-reporting/internal/http"
	"github.com/nexus/citizen-reporting/internal/store"
	"github.com/nexus/citizen-reporting/migrations"
)

func main() {
	ctx := context.Background()
	app := httpx.New("citizen-reporting")

	pool, err := pg.Connect(ctx, config.String("DATABASE_URL", "postgres://nexus:nexus@localhost:5432/reports?sslmode=disable"), 90*time.Second, app.Log)
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
	b := bus.Connect(ctx, config.RabbitURL(), "citizen-reporting", app.Log)
	b.Subscribe("q.citizen-reporting", cbus.Keys, (&cbus.Consumer{Store: st}).Handle)

	app.Ready("postgres", func(c context.Context) error { return pool.Ping(c) })
	app.Ready("rabbitmq", b.Check)
	(&api.API{Store: st, Pub: b, Limiter: ratelimit.New(config.RedisURL()), Secret: config.JWTSecret()}).Register(app)

	if err := app.Run(ctx); err != nil {
		app.Log.Error("server", "err", err)
		os.Exit(1)
	}
}
