// Package store is the Postgres access layer for environmental data. Documented records (rainfall of
// 15-16 Sep 2025, river watch, flood-prone localities, landslide points, zoning) are seeded from the
// frontend datasets and keep their source key; live readings from sensors or the simulator are labelled by their source.
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"nexus/shared/seed"
)

type Store struct{ DB *pgxpool.Pool }

func (s *Store) Empty(ctx context.Context) (bool, error) {
	var e bool
	return e, s.DB.QueryRow(ctx, `SELECT NOT EXISTS (SELECT 1 FROM flood_zones)`).Scan(&e)
}

func (s *Store) Seed(ctx context.Context, dir string) error {
	var rain struct {
		Label string  `json:"label"`
		S     string  `json:"s"`
		Rows  [][]any `json:"rows"`
	}
	var zoning struct {
		Label string  `json:"label"`
		S     string  `json:"s"`
		Rows  [][]any `json:"rows"`
	}
	var rivers []struct {
		R, At, D, St, Lv, S string
	}
	var flood []struct {
		ID, N, R, Note, Node string
		Lat, Lng             float64
		S                    []string
	}
	var slides []struct {
		ID, N, Road, Note string
		Lat, Lng          float64
		S                 []string
	}
	for name, v := range map[string]any{"RAIN": &rain, "ZONING": &zoning, "RIVERWATCH": &rivers, "FLOOD": &flood, "SLIDES": &slides} {
		if err := seed.Load(dir, name, v); err != nil {
			return err
		}
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, r := range rain.Rows {
		mm, _ := r[1].(float64)
		if _, err := tx.Exec(ctx, `INSERT INTO rain_obs (station,mm,label,source) VALUES ($1,$2,$3,$4)`, r[0], mm, rain.Label, rain.S); err != nil {
			return err
		}
	}
	for _, r := range rivers {
		if _, err := tx.Exec(ctx, `INSERT INTO river_levels (river,gauge,status,lv,observed,source) VALUES ($1,$2,$3,$4,$5,$6)`, r.R, r.At, r.St, r.Lv, r.D, r.S); err != nil {
			return err
		}
	}
	for _, f := range flood {
		if _, err := tx.Exec(ctx, `INSERT INTO flood_zones (id,name,river,lat,lng,note,node,sources) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, f.ID, f.N, f.R, f.Lat, f.Lng, f.Note, f.Node, f.S); err != nil {
			return err
		}
	}
	for _, l := range slides {
		if _, err := tx.Exec(ctx, `INSERT INTO landslides (id,name,road,lat,lng,note,sources) VALUES ($1,$2,$3,$4,$5,$6,$7)`, l.ID, l.N, l.Road, l.Lat, l.Lng, l.Note, l.S); err != nil {
			return err
		}
	}
	for _, z := range zoning.Rows {
		share, _ := z[1].(float64)
		if _, err := tx.Exec(ctx, `INSERT INTO zoning (class,share,tone,label,source) VALUES ($1,$2,$3,$4,$5)`, z[0], share, z[2], zoning.Label, zoning.S); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO flood_state (stage, source) VALUES (0, 'seed')`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// query runs a SELECT and returns each row as a JSON object (columns become keys).
func (s *Store) query(ctx context.Context, q string, args ...any) ([]map[string]any, error) {
	rows, err := s.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := rows.FieldDescriptions()
	out := []map[string]any{}
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		m := map[string]any{}
		for i, c := range cols {
			m[c.Name] = vals[i]
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) Rain(ctx context.Context) ([]map[string]any, error) {
	return s.query(ctx, `SELECT station, mm, at, label, source FROM rain_obs ORDER BY at DESC, id DESC LIMIT 200`)
}
func (s *Store) Rivers(ctx context.Context) ([]map[string]any, error) {
	return s.query(ctx, `SELECT river, gauge, level, danger, status, lv, observed, at, source FROM river_levels ORDER BY at DESC, id DESC LIMIT 200`)
}
func (s *Store) FloodZones(ctx context.Context) ([]map[string]any, error) {
	return s.query(ctx, `SELECT id, name, river, lat, lng, note, node, sources, source FROM flood_zones ORDER BY id`)
}
func (s *Store) Landslides(ctx context.Context) ([]map[string]any, error) {
	return s.query(ctx, `SELECT id, name, road, lat, lng, note, sources, source FROM landslides ORDER BY id`)
}
func (s *Store) Zoning(ctx context.Context) ([]map[string]any, error) {
	return s.query(ctx, `SELECT class, share, tone, label, source FROM zoning ORDER BY share DESC`)
}

func (s *Store) FloodStage(ctx context.Context) (float64, string, error) {
	var st float64
	var src string
	err := s.DB.QueryRow(ctx, `SELECT stage, source FROM flood_state ORDER BY id DESC LIMIT 1`).Scan(&st, &src)
	return st, src, err
}

func (s *Store) SetFloodStage(ctx context.Context, stage float64, source string) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO flood_state (stage, source) VALUES ($1,$2)`, stage, source)
	return err
}

func (s *Store) AddRain(ctx context.Context, station string, mm float64, source string) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO rain_obs (station,mm,label,source) VALUES ($1,$2,'live reading',$3)`, station, mm, source)
	return err
}

func (s *Store) AddRiver(ctx context.Context, gauge string, level, danger float64, status, source string) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO river_levels (river,gauge,level,danger,status,lv,observed,source) VALUES ($1,$1,$2,$3,$4,$5,'live reading',$6)`,
		gauge, level, danger, status, map[string]string{"below": "ok", "warning": "warn", "danger": "bad"}[status], source)
	return err
}
