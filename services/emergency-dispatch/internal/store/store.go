// Package store is the Postgres access layer for dispatch: hospitals, units, shelters,
// pharmacies, stock and assignments. Seeded on first boot from the exported frontend datasets.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nexus/shared/seed"

	"github.com/nexus/emergency-dispatch/internal/domain"
)

type Store struct{ DB *pgxpool.Pool }

var ErrNotFound = errors.New("not found")

// Seed loads the registry and the simulated state. With force it wipes first (scenario reload / reset).
func (s *Store) Seed(ctx context.Context, dir string, force bool) error {
	var empty bool
	if err := s.DB.QueryRow(ctx, `SELECT NOT EXISTS (SELECT 1 FROM hospitals)`).Scan(&empty); err != nil {
		return err
	}
	if !empty && !force {
		return nil
	}
	var hs []seed.Hospital
	var simcap map[string]int
	var shelters []seed.Shelter
	var ps []seed.Pharmacy
	var ms []seed.Medicine
	var nodes map[string][]json.RawMessage
	var units []seed.Unit
	for name, v := range map[string]any{"HOSPITALS": &hs, "SIMCAP": &simcap, "SHELTERS": &shelters, "PHARMACIES": &ps, "MEDICINES": &ms, "NODES": &nodes, "UNITS": &units} {
		if err := seed.Load(dir, name, v); err != nil {
			return err
		}
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `TRUNCATE assignments, stock, pharmacies, shelters, units, hospitals`); err != nil {
		return err
	}
	state := seed.SeedHosp(hs, simcap) // free beds from SIMCAP, same as the offline console
	for _, h := range hs {
		st := state[h.ID]
		if _, err := tx.Exec(ctx, `INSERT INTO hospitals (id,name,own,lvl,lat,lng,loc,phone,beds,node,cap,free,inbound,divert,source) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,0,false,'nic')`,
			h.ID, h.N, h.Own, h.Lvl, h.Lat, h.Lng, h.Loc, h.Ph, h.Beds, h.Node, st.Cap, st.Free); err != nil {
			return err
		}
	}
	for _, u := range units {
		var lat, lng float64
		if n, ok := nodes[u.Node]; ok && len(n) >= 2 {
			_ = json.Unmarshal(n[0], &lat)
			_ = json.Unmarshal(n[1], &lng)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO units (id,type,node,status,task,lat,lng) VALUES ($1,$2,$3,'Available','',$4,$5)`, u.ID, u.Type, u.Node, lat, lng); err != nil {
			return err
		}
	}
	for _, x := range shelters {
		if _, err := tx.Exec(ctx, `INSERT INTO shelters (id,name,node,lat,lng,cap,occ,open) VALUES ($1,$2,$3,$4,$5,$6,$7,true)`, x.ID, x.N, x.Node, x.Lat, x.Lng, x.Cap, x.Occ); err != nil {
			return err
		}
	}
	stock := seed.SeedStock(ps, ms)
	for _, p := range ps {
		if _, err := tx.Exec(ctx, `INSERT INTO pharmacies (id,name,lat,lng,loc,hrs) VALUES ($1,$2,$3,$4,$5,$6)`, p.ID, p.N, p.Lat, p.Lng, p.Loc, p.Hrs); err != nil {
			return err
		}
		for _, m := range ms {
			if _, err := tx.Exec(ctx, `INSERT INTO stock (pharmacy_id,med_id,qty) VALUES ($1,$2,$3)`, p.ID, m.ID, stock[p.ID][m.ID]); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

// ---- hospitals ----

const hospCols = `id,name,own,lvl,lat,lng,loc,phone,beds,node,cap,free,inbound,divert,source`

func scanHosp(r pgx.Row) (domain.Hospital, error) {
	var h domain.Hospital
	var loc, ph, node *string
	err := r.Scan(&h.ID, &h.N, &h.Own, &h.Lvl, &h.Lat, &h.Lng, &loc, &ph, &h.Beds, &node, &h.Cap, &h.Free, &h.Inbound, &h.Divert, &h.Source)
	if loc != nil {
		h.Loc = *loc
	}
	if ph != nil {
		h.Ph = *ph
	}
	if node != nil {
		h.Node = *node
	}
	h.CapSrc = "simulated"
	return h, err
}

func (s *Store) Hospitals(ctx context.Context) ([]domain.Hospital, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+hospCols+` FROM hospitals ORDER BY ctid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Hospital
	for rows.Next() {
		h, err := scanHosp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) Hospital(ctx context.Context, id string) (domain.Hospital, error) {
	h, err := scanHosp(s.DB.QueryRow(ctx, `SELECT `+hospCols+` FROM hospitals WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return h, ErrNotFound
	}
	return h, err
}

// PatchHospital sets whichever of free, inbound, divert are non-nil (absolute values).
func (s *Store) PatchHospital(ctx context.Context, id string, free, inbound *int, divert *bool) (domain.Hospital, error) {
	h, err := scanHosp(s.DB.QueryRow(ctx, `UPDATE hospitals SET free=COALESCE(LEAST($2::int, cap), free), inbound=COALESCE($3::int, inbound), divert=COALESCE($4::boolean, divert) WHERE id=$1 RETURNING `+hospCols, id, free, inbound, divert))
	if errors.Is(err, pgx.ErrNoRows) {
		return h, ErrNotFound
	}
	return h, err
}

func (s *Store) BumpInbound(ctx context.Context, id string, delta int) (domain.Hospital, error) {
	h, err := scanHosp(s.DB.QueryRow(ctx, `UPDATE hospitals SET inbound=GREATEST(0, inbound+$2) WHERE id=$1 RETURNING `+hospCols, id, delta))
	if errors.Is(err, pgx.ErrNoRows) {
		return h, ErrNotFound
	}
	return h, err
}

// ---- units ----

func scanUnit(r pgx.Row) (domain.Unit, error) {
	var u domain.Unit
	var node *string
	err := r.Scan(&u.ID, &u.Type, &node, &u.Status, &u.Task, &u.Lat, &u.Lng, &u.Step)
	if node != nil {
		u.Node = *node
	}
	return u, err
}

const unitCols = `id,type,node,status,task,lat,lng,step`

func (s *Store) Units(ctx context.Context) ([]domain.Unit, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+unitCols+` FROM units ORDER BY ctid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Unit
	for rows.Next() {
		u, err := scanUnit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) SetUnit(ctx context.Context, id, status string, task, step *string) (domain.Unit, error) {
	u, err := scanUnit(s.DB.QueryRow(ctx, `UPDATE units SET status=$2, task=COALESCE($3, CASE WHEN $2='Available' THEN '' ELSE task END), step=CASE WHEN $2='Available' THEN NULL ELSE COALESCE($4, step) END WHERE id=$1 RETURNING `+unitCols, id, status, task, step))
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

func (s *Store) SetUnitStep(ctx context.Context, id, step string) (domain.Unit, error) {
	u, err := scanUnit(s.DB.QueryRow(ctx, `UPDATE units SET step=CASE WHEN $2='Free' THEN NULL ELSE $2 END, status=CASE WHEN $2='Free' THEN 'Available' ELSE status END, task=CASE WHEN $2='Free' THEN '' ELSE task END WHERE id=$1 RETURNING `+unitCols, id, step))
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

func (s *Store) AllAvailable(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `UPDATE units SET status='Available', task='', step=NULL`)
	return err
}

// ---- shelters ----

func (s *Store) Shelters(ctx context.Context) ([]domain.Shelter, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,name,node,lat,lng,cap,occ,open FROM shelters ORDER BY ctid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Shelter
	for rows.Next() {
		var x domain.Shelter
		var node *string
		if err := rows.Scan(&x.ID, &x.N, &node, &x.Lat, &x.Lng, &x.Cap, &x.Occ, &x.Open); err != nil {
			return nil, err
		}
		if node != nil {
			x.Node = *node
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) PatchShelter(ctx context.Context, id string, occ *int, open *bool) (domain.Shelter, error) {
	var x domain.Shelter
	var node *string
	err := s.DB.QueryRow(ctx, `UPDATE shelters SET occ=COALESCE(GREATEST(0, LEAST($2::int, cap)), occ), open=COALESCE($3::boolean, open) WHERE id=$1 RETURNING id,name,node,lat,lng,cap,occ,open`, id, occ, open).
		Scan(&x.ID, &x.N, &node, &x.Lat, &x.Lng, &x.Cap, &x.Occ, &x.Open)
	if errors.Is(err, pgx.ErrNoRows) {
		return x, ErrNotFound
	}
	if node != nil {
		x.Node = *node
	}
	return x, err
}

// AddOccupancy adds people to a shelter, capped at capacity.
func (s *Store) AddOccupancy(ctx context.Context, id string, people int) (domain.Shelter, error) {
	var cur int
	if err := s.DB.QueryRow(ctx, `SELECT occ FROM shelters WHERE id=$1`, id).Scan(&cur); err != nil {
		return domain.Shelter{}, ErrNotFound
	}
	n := cur + people
	return s.PatchShelter(ctx, id, &n, nil)
}

// ---- pharmacies ----

func (s *Store) Pharmacies(ctx context.Context) ([]domain.Pharmacy, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,name,lat,lng,COALESCE(loc,''),COALESCE(hrs,'') FROM pharmacies ORDER BY ctid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Pharmacy
	idx := map[string]int{}
	for rows.Next() {
		var p domain.Pharmacy
		if err := rows.Scan(&p.ID, &p.N, &p.Lat, &p.Lng, &p.Loc, &p.Hrs); err != nil {
			return nil, err
		}
		p.Stock = map[string]int{}
		idx[p.ID] = len(out)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sr, err := s.DB.Query(ctx, `SELECT pharmacy_id, med_id, qty FROM stock`)
	if err != nil {
		return nil, err
	}
	defer sr.Close()
	for sr.Next() {
		var pid, mid string
		var q int
		if err := sr.Scan(&pid, &mid, &q); err != nil {
			return nil, err
		}
		if i, ok := idx[pid]; ok {
			out[i].Stock[mid] = q
		}
	}
	return out, sr.Err()
}

func (s *Store) SetStock(ctx context.Context, pid, med string, qty int) error {
	if qty < 0 {
		qty = 0
	}
	ct, err := s.DB.Exec(ctx, `UPDATE stock SET qty=$3 WHERE pharmacy_id=$1 AND med_id=$2`, pid, med, qty)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- assignments ----

type Assignment struct {
	ID         string          `json:"id"`
	IncidentID string          `json:"incident_id"`
	UnitID     string          `json:"unit_id"`
	HospitalID string          `json:"hospital_id"`
	Route      json.RawMessage `json:"route"`
	ApprovedAt string          `json:"approved_at"`
}

func (s *Store) InsertAssignment(ctx context.Context, a Assignment) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO assignments (id,incident_id,unit_id,hospital_id,route) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (id) DO NOTHING`, a.ID, a.IncidentID, a.UnitID, a.HospitalID, nullJSON(a.Route))
	return err
}

func nullJSON(b json.RawMessage) any {
	if len(b) == 0 {
		return nil
	}
	return []byte(b)
}

func (s *Store) Assignments(ctx context.Context, limit int) ([]Assignment, error) {
	rows, err := s.DB.Query(ctx, fmt.Sprintf(`SELECT id,COALESCE(incident_id,''),COALESCE(unit_id,''),COALESCE(hospital_id,''),route,approved_at::text FROM assignments ORDER BY approved_at DESC LIMIT %d`, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Assignment{}
	for rows.Next() {
		var a Assignment
		var route []byte
		if err := rows.Scan(&a.ID, &a.IncidentID, &a.UnitID, &a.HospitalID, &route, &a.ApprovedAt); err != nil {
			return nil, err
		}
		a.Route = route
		out = append(out, a)
	}
	return out, rows.Err()
}

// ClaimUnit atomically takes an Available unit; false means someone else got it first.
func (s *Store) ClaimUnit(ctx context.Context, id, task string) (bool, error) {
	ct, err := s.DB.Exec(ctx, `UPDATE units SET status='On mission', task=$2 WHERE id=$1 AND status='Available'`, id, task)
	return ct.RowsAffected() == 1, err
}

func (s *Store) Unit(ctx context.Context, id string) (domain.Unit, error) {
	u, err := scanUnit(s.DB.QueryRow(ctx, `SELECT `+unitCols+` FROM units WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

func (s *Store) Shelter(ctx context.Context, id string) (domain.Shelter, error) {
	list, err := s.Shelters(ctx)
	if err != nil {
		return domain.Shelter{}, err
	}
	for _, x := range list {
		if x.ID == id {
			return x, nil
		}
	}
	return domain.Shelter{}, ErrNotFound
}
