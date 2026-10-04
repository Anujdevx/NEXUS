// Package store is the Postgres access layer for identity.
package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexus/identity-access/internal/domain"
)

type Store struct{ DB *pgxpool.Pool }

type Row struct {
	domain.User
	Hash string
}

func (s *Store) ByEmail(ctx context.Context, email string) (*Row, error) {
	var r Row
	err := s.DB.QueryRow(ctx, `SELECT id::text, email, role, password_hash FROM users WHERE lower(email)=lower($1)`, email).Scan(&r.ID, &r.Email, &r.Role, &r.Hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &r, err
}

// Ensure creates the demo user, or resets its password and role to what the environment says (so editing .env takes effect on restart).
func (s *Store) Ensure(ctx context.Context, email, hash, role string) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO users(id,email,password_hash,role) VALUES($1,$2,$3,$4) ON CONFLICT (email) DO UPDATE SET password_hash=EXCLUDED.password_hash, role=EXCLUDED.role`, uuid.NewString(), email, hash, role)
	return err
}

func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	return n, s.DB.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
}
