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

	"github.com/nexus/identity-access/internal/domain"
	api "github.com/nexus/identity-access/internal/http"
	"github.com/nexus/identity-access/internal/store"
	"github.com/nexus/identity-access/migrations"
)

func main() {
	ctx := context.Background()
	app := httpx.New("identity-access")

	pool, err := pg.Connect(ctx, config.String("DATABASE_URL", "postgres://nexus:nexus@localhost:5432/identity?sslmode=disable"), 90*time.Second, app.Log)
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
	seedDemoUsers(ctx, st, app)

	b := bus.Connect(ctx, config.RabbitURL(), "identity-access", app.Log)
	app.Ready("postgres", func(c context.Context) error { return pool.Ping(c) })
	app.Ready("rabbitmq", b.Check)

	(&api.API{
		Store: st, Bus: b, Limiter: ratelimit.New(config.RedisURL()), Secret: config.JWTSecret(),
		DemoMode: config.Bool("DEMO_MODE", true), TTL: config.Duration("TOKEN_TTL", 12*time.Hour),
	}).Register(app)

	if err := app.Run(ctx); err != nil {
		app.Log.Error("server", "err", err)
		os.Exit(1)
	}
}

// seedDemoUsers creates the three demo accounts from env (passwords are not in git).
func seedDemoUsers(ctx context.Context, st *store.Store, app *httpx.App) {
	for _, u := range []struct{ email, role, env string }{
		{"controller@nexus.local", "Controller", "DEMO_CONTROLLER_PASSWORD"},
		{"responder@nexus.local", "Responder", "DEMO_RESPONDER_PASSWORD"},
		{"citizen@nexus.local", "Citizen", "DEMO_CITIZEN_PASSWORD"},
	} {
		h, err := domain.HashPassword(config.String(u.env, "nexus-demo"))
		if err == nil {
			err = st.Ensure(ctx, u.email, h, u.role)
		}
		if err != nil {
			app.Log.Warn("seed user", "email", u.email, "err", err)
		}
	}
}
