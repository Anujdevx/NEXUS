// Package store is the Postgres access layer for road closures and reported breaks.
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nexus/shared/seed"
)

type Store struct{ DB *pgxpool.Pool }

var ErrNotFound = errors.New("not found")

type Closure struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Kind   string  `json:"kind"`
	Note   string  `json:"note"`
	Source string  `json:"source"`
	Lat    float64 `json:"lat"`
	Lng    float64 `json:"lng"`
	Active bool    `json:"active"`
}

type Break struct {
	EdgeID    string  `json:"edgeId"`
	Reason    string  `json:"reason"`
	CreatedBy string  `json:"created_by"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Name      string  `json:"name"`
	CreatedAt string  `json:"created_at"`
}

// Seed loads the documented closures (CLOSURES in the frontend datasets) and activates those of the given scenario.
func (s *Store) Seed(ctx context.Context, dir, scenario string) error {
	var cl map[string]seed.Closure
	var sc map[string]seed.Scenario
	if err := seed.Load(dir, "CLOSURES", &cl); err != nil {
		return err
	}
	if err := seed.Load(dir, "SCENARIOS", &sc); err != nil {
		return err
	}
	on := map[string]bool{}
	for _, id := range sc[scenario].Closures {
		on[id] = true
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for id, c := range cl {
		if _, err := tx.Exec(ctx, `INSERT INTO closures (id,name,kind,note,source,lat,lng,active) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (id) DO UPDATE SET name=$2, kind=$3, note=$4, source=$5, lat=$6, lng=$7`, id, c.N, c.Kind, c.Note, c.S, c.At[0], c.At[1], on[id]); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) Empty(ctx context.Context) (bool, error) {
	var e bool
	return e, s.DB.QueryRow(ctx, `SELECT NOT EXISTS (SELECT 1 FROM closures)`).Scan(&e)
}

// ResetScenario activates exactly the scenario's closures and clears every break.
func (s *Store) ResetScenario(ctx context.Context, closures []string) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE closures SET active = (id = ANY($1))`, closures); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM breaks`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Closures(ctx context.Context) ([]Closure, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,name,kind,COALESCE(note,''),COALESCE(source,''),COALESCE(lat,0),COALESCE(lng,0),active FROM closures ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Closure{}
	for rows.Next() {
		var c Closure
		if err := rows.Scan(&c.ID, &c.Name, &c.Kind, &c.Note, &c.Source, &c.Lat, &c.Lng, &c.Active); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) SetClosure(ctx context.Context, id string, active *bool) (Closure, error) {
	var c Closure
	err := s.DB.QueryRow(ctx, `UPDATE closures SET active = COALESCE($2, NOT active) WHERE id=$1 RETURNING id,name,kind,COALESCE(note,''),COALESCE(source,''),COALESCE(lat,0),COALESCE(lng,0),active`, id, active).
		Scan(&c.ID, &c.Name, &c.Kind, &c.Note, &c.Source, &c.Lat, &c.Lng, &c.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

func (s *Store) AddBreak(ctx context.Context, b Break) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO breaks (edge_id,reason,created_by,lat,lng,name) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (edge_id) DO UPDATE SET reason=$2, lat=$4, lng=$5, name=$6`, b.EdgeID, b.Reason, b.CreatedBy, b.Lat, b.Lng, b.Name)
	return err
}

func (s *Store) RemoveBreak(ctx context.Context, edgeID string) (*Break, error) {
	var b Break
	err := s.DB.QueryRow(ctx, `DELETE FROM breaks WHERE edge_id=$1 RETURNING edge_id,COALESCE(reason,''),COALESCE(created_by,''),lat,lng,COALESCE(name,'')`, edgeID).
		Scan(&b.EdgeID, &b.Reason, &b.CreatedBy, &b.Lat, &b.Lng, &b.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &b, err
}

// RemoveBreakNear removes the break closest to a point (the console removes breaks by list position).
func (s *Store) RemoveBreakNear(ctx context.Context, lat, lng float64) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM breaks WHERE edge_id = (SELECT edge_id FROM breaks ORDER BY (lat-$1)^2 + (lng-$2)^2 LIMIT 1)`, lat, lng)
	return err
}

func (s *Store) ClearBreaks(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM breaks`)
	return err
}

func (s *Store) Breaks(ctx context.Context) ([]Break, error) {
	rows, err := s.DB.Query(ctx, `SELECT edge_id,COALESCE(reason,''),COALESCE(created_by,''),lat,lng,COALESCE(name,''),created_at::text FROM breaks ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Break{}
	for rows.Next() {
		var b Break
		if err := rows.Scan(&b.EdgeID, &b.Reason, &b.CreatedBy, &b.Lat, &b.Lng, &b.Name, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
