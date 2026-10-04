// Package store writes the server-side audit trail: every envelope seen on the bus.
package store

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nexus/shared/envelope"
)

type Store struct {
	DB  *pgxpool.Pool
	Log *slog.Logger
	in  chan envelope.Envelope
}

func New(db *pgxpool.Pool, log *slog.Logger) *Store {
	return &Store{DB: db, Log: log, in: make(chan envelope.Envelope, 4096)}
}

// Log queues an envelope for writing; it never blocks the bus consumer.
func (s *Store) Record(env envelope.Envelope) {
	select {
	case s.in <- env:
	default:
		s.Log.Warn("event log queue full, dropping envelope", "key", env.Key())
	}
}

// Run batches queued envelopes into the event_log table until ctx ends.
func (s *Store) Run(ctx context.Context) {
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	var batch []envelope.Envelope
	flush := func() {
		if len(batch) == 0 {
			return
		}
		b := &pgx.Batch{}
		for _, e := range batch {
			body, _ := json.Marshal(e)
			b.Queue(`INSERT INTO event_log (id, routing_key, envelope) VALUES ($1::uuid, $2, $3) ON CONFLICT (id) DO NOTHING`, e.ID, e.Key(), body)
		}
		fctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		res := s.DB.SendBatch(fctx, b)
		for range batch {
			if _, err := res.Exec(); err != nil {
				s.Log.Warn("event log insert", "err", err)
				break
			}
		}
		_ = res.Close()
		batch = batch[:0]
	}
	for {
		select {
		case e := <-s.in:
			batch = append(batch, e)
			if len(batch) >= 100 {
				flush()
			}
		case <-tick.C:
			flush()
		case <-ctx.Done():
			flush()
			return
		}
	}
}

// Recent returns the newest envelopes first.
func (s *Store) Recent(ctx context.Context, limit int, key string) ([]json.RawMessage, error) {
	q := `SELECT envelope FROM event_log ORDER BY received_at DESC LIMIT $1`
	args := []any{limit}
	if key != "" {
		q = `SELECT envelope FROM event_log WHERE routing_key LIKE $2 ORDER BY received_at DESC LIMIT $1`
		args = append(args, key+"%")
	}
	rows, err := s.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
