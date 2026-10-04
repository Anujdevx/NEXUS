CREATE TABLE IF NOT EXISTS closures (
  id text PRIMARY KEY, name text NOT NULL, kind text NOT NULL, note text, source text,
  lat double precision, lng double precision, active boolean NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS breaks (
  edge_id text PRIMARY KEY, reason text, created_at timestamptz NOT NULL DEFAULT now(), created_by text,
  lat double precision NOT NULL, lng double precision NOT NULL, name text
);
