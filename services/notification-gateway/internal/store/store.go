// Package store persists issued alerts.
package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexus/notification-gateway/internal/domain"
)

type Store struct{ DB *pgxpool.Pool }

func (s *Store) Save(ctx context.Context, a domain.Alert) error {
	cap, _ := json.Marshal(a.CAP)
	_, err := s.DB.Exec(ctx, `INSERT INTO alerts (id,area,severity,text_en,text_hi,issued_at,issued_by,source,cap) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT (id) DO NOTHING`,
		a.ID, a.Area, a.Severity, a.TextEN, a.TextHI, a.IssuedAt, a.IssuedBy, a.Source, cap)
	return err
}

func (s *Store) List(ctx context.Context, limit int) ([]domain.Alert, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,area,severity,text_en,text_hi,issued_at,issued_by,source,cap FROM alerts ORDER BY issued_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Alert{}
	for rows.Next() {
		var a domain.Alert
		var cap []byte
		var at time.Time
		if err := rows.Scan(&a.ID, &a.Area, &a.Severity, &a.TextEN, &a.TextHI, &at, &a.IssuedBy, &a.Source, &cap); err != nil {
			return nil, err
		}
		a.IssuedAt = at
		_ = json.Unmarshal(cap, &a.CAP)
		out = append(out, a)
	}
	return out, rows.Err()
}
