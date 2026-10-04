package main

import (
	"context"
	"os"
	"time"

	"nexus/shared/bus"
	"nexus/shared/config"
	"nexus/shared/httpx"
	"nexus/shared/pg"

	nbus "github.com/nexus/notification-gateway/internal/bus"
	api "github.com/nexus/notification-gateway/internal/http"
	"github.com/nexus/notification-gateway/internal/store"
	"github.com/nexus/notification-gateway/migrations"
)

func main() {
	ctx := context.Background()
	app := httpx.New("notification-gateway")
	pool, err := pg.Connect(ctx, config.String("DATABASE_URL", "postgres://nexus:nexus@localhost:5432/alerts?sslmode=disable"), 90*time.Second, app.Log)
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
	b := bus.Connect(ctx, config.RabbitURL(), "notification-gateway", app.Log)
	a := &api.API{Store: st, Pub: b, Secret: config.JWTSecret(), Key: config.ServiceKey()}
	b.Subscribe("q.notification-gateway", nbus.Keys, nbus.New(a, st, app.Log).Handle)
	app.Ready("postgres", func(c context.Context) error { return pool.Ping(c) })
	app.Ready("rabbitmq", b.Check)
	a.Register(app)
	if err := app.Run(ctx); err != nil {
		app.Log.Error("server", "err", err)
		os.Exit(1)
	}
}
