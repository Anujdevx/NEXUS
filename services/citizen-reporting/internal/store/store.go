// Package store is the Postgres access layer for citizen reports and incidents.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexus/citizen-reporting/internal/domain"
)

type Store struct{ DB *pgxpool.Pool }

var ErrNotFound = errors.New("not found")

// SaveSOS stores the SOS and its incident in one transaction.
func (s *Store) SaveSOS(ctx context.Context, x domain.SOS, inc domain.Incident) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO sos (id,place,hazard,people,injured,lat,lng,via,status,client) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'received',$9) ON CONFLICT (id) DO NOTHING`,
		x.ID, x.PlaceName, x.Hazard, x.People, x.Injured == "Yes", x.Lat, x.Lng, x.Via, x.Client); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO incidents (id,kind,title,place,sev,lat,lng,node,status,sim,scenario,detail) VALUES ($1,'SOS',$2,$3,$4,$5,$6,$7,'Open',true,NULL,$8) ON CONFLICT (id) DO NOTHING`,
		inc.ID, inc.N, inc.Place, inc.Sev, inc.Lat, inc.Lng, inc.Node, inc.D); err != nil {
		return err
	}
	for _, l := range inc.Log {
		if _, err := tx.Exec(ctx, `INSERT INTO incident_log (incident_id, text) VALUES ($1,$2)`, inc.ID, l[0]); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) AddLog(ctx context.Context, id, text string) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO incident_log (incident_id, text) VALUES ($1,$2)`, id, text)
	return err
}

// SetStatus updates an incident's status (and unit when given). It reports whether the incident exists.
func (s *Store) SetStatus(ctx context.Context, id, status string, unit *string) (bool, error) {
	ct, err := s.DB.Exec(ctx, `UPDATE incidents SET status=$2, unit_id=COALESCE($3, unit_id) WHERE id=$1`, id, status, unit)
	return ct.RowsAffected() == 1, err
}

func (s *Store) SetSOSStatus(ctx context.Context, id, status string) error {
	_, err := s.DB.Exec(ctx, `UPDATE sos SET status=$2 WHERE id=$1`, id, status)
	return err
}

func (s *Store) Incidents(ctx context.Context, limit int) ([]domain.Incident, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,kind,title,COALESCE(place,''),COALESCE(sev,''),COALESCE(lat,0),COALESCE(lng,0),COALESCE(node,''),status,unit_id,sim,COALESCE(detail,''),created_at FROM incidents ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Incident{}
	idx := map[string]int{}
	for rows.Next() {
		var i domain.Incident
		var created time.Time
		if err := rows.Scan(&i.ID, &i.T, &i.N, &i.Place, &i.Sev, &i.Lat, &i.Lng, &i.Node, &i.Status, &i.Unit, &i.Sim, &i.D, &created); err != nil {
			return nil, err
		}
		i.S, i.Log, i.When = []string{}, [][]string{}, "earlier"
		idx[i.ID] = len(out)
		out = append(out, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	lr, err := s.DB.Query(ctx, `SELECT incident_id, text, at FROM incident_log ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer lr.Close()
	for lr.Next() {
		var id, text string
		var at time.Time
		if err := lr.Scan(&id, &text, &at); err != nil {
			return nil, err
		}
		if k, ok := idx[id]; ok {
			out[k].Log = append(out[k].Log, []string{text, at.Local().Format("03:04 PM")})
		}
	}
	return out, lr.Err()
}

func (s *Store) Incident(ctx context.Context, id string) (*domain.Incident, error) {
	all, err := s.Incidents(ctx, 500)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].ID == id {
			return &all[i], nil
		}
	}
	return nil, ErrNotFound
}

type Report struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	Description string  `json:"description"`
	MediaID     string  `json:"media_id"`
	UserID      string  `json:"user_id"`
	Status      string  `json:"status"`
}

func (s *Store) SaveReport(ctx context.Context, r Report) (Report, error) {
	r.ID, r.Status = uuid.NewString(), "new"
	_, err := s.DB.Exec(ctx, `INSERT INTO reports (id,user_id,type,lat,lng,description,media_id) VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''))`, r.ID, r.UserID, r.Type, r.Lat, r.Lng, r.Description, r.MediaID)
	return r, err
}

func (s *Store) Reports(ctx context.Context, limit int) ([]Report, error) {
	rows, err := s.DB.Query(ctx, `SELECT id::text, COALESCE(user_id,''), type, COALESCE(lat,0), COALESCE(lng,0), COALESCE(description,''), COALESCE(media_id,''), status FROM reports ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Report{}
	for rows.Next() {
		var r Report
		if err := rows.Scan(&r.ID, &r.UserID, &r.Type, &r.Lat, &r.Lng, &r.Description, &r.MediaID, &r.Status); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

var _ = pgx.ErrNoRows
