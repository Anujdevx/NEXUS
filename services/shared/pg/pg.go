// Package pg connects to Postgres with retry and runs embedded SQL migrations on boot.
package pg

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect retries for up to wait so a service can boot before its database is up.
func Connect(ctx context.Context, url string, wait time.Duration, log *slog.Logger) (*pgxpool.Pool, error) {
	deadline := time.Now().Add(wait)
	var last error
	for {
		pool, err := pgxpool.New(ctx, url)
		if err == nil {
			pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err = pool.Ping(pctx)
			cancel()
			if err == nil {
				return pool, nil
			}
			pool.Close()
		}
		last = err
		if time.Now().After(deadline) || ctx.Err() != nil {
			return nil, fmt.Errorf("postgres unavailable: %w", last)
		}
		log.Info("waiting for postgres", "err", last.Error())
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Migrate applies every *.sql in dir of fsys, in name order, once each.
func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS, dir string, log *slog.Logger) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return err
	}
	names := []string{}
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > 4 && e.Name()[len(e.Name())-4:] == ".sql" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		var done bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)`, n).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		b, err := fs.ReadFile(fsys, path.Join(dir, n))
		if err != nil {
			return err
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(b)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", n, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(name) VALUES($1)`, n); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		log.Info("migration applied", "name", n)
	}
	return nil
}

// Empty reports whether a table has no rows (used to seed on first boot).
func Empty(ctx context.Context, pool *pgxpool.Pool, table string) (bool, error) {
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		return false, err
	}
	return n == 0, nil
}
