package main

import (
	"context"
	"os"
	"time"

	"nexus/shared/bus"
	"nexus/shared/config"
	"nexus/shared/envelope"
	"nexus/shared/httpx"
	"nexus/shared/pg"

	api "github.com/nexus/iot-broker/internal/http"
	"github.com/nexus/iot-broker/internal/hub"
	"github.com/nexus/iot-broker/internal/store"
	"github.com/nexus/iot-broker/migrations"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := httpx.New("iot-broker")

	pool, err := pg.Connect(ctx, config.String("DATABASE_URL", "postgres://nexus:nexus@localhost:5432/events?sslmode=disable"), 90*time.Second, app.Log)
	if err != nil {
		app.Log.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pg.Migrate(ctx, pool, migrations.FS, ".", app.Log); err != nil {
		app.Log.Error("migrate", "err", err)
		os.Exit(1)
	}

	st := store.New(pool, app.Log)
	go st.Run(ctx)
	h := hub.New(app.Log)
	b := bus.Connect(ctx, config.RabbitURL(), "iot-broker", app.Log)
	// "#": every envelope, for the audit log and the WebSocket fan-out. Includes our own telemetry.raw.
	b.Subscribe("q.iot-broker", []string{"#"}, func(_ context.Context, env envelope.Envelope) error {
		st.Record(env)
		h.Broadcast(env)
		return nil
	}, bus.IncludeOwn())

	app.Ready("postgres", func(c context.Context) error { return pool.Ping(c) })
	app.Ready("rabbitmq", b.Check)
	api.New(b, st, h, config.JWTSecret(), config.ServiceKey()).Register(app)

	if err := app.Run(ctx); err != nil {
		app.Log.Error("server", "err", err)
		os.Exit(1)
	}
}
