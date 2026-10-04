CREATE TABLE IF NOT EXISTS rain_obs (
  id bigserial PRIMARY KEY, station text NOT NULL, mm double precision NOT NULL, at timestamptz NOT NULL DEFAULT now(),
  label text, source text NOT NULL
);
CREATE INDEX IF NOT EXISTS rain_obs_at_idx ON rain_obs (at DESC);
CREATE TABLE IF NOT EXISTS river_levels (
  id bigserial PRIMARY KEY, river text NOT NULL, gauge text NOT NULL, level double precision, danger double precision,
  status text, lv text, at timestamptz NOT NULL DEFAULT now(), observed text, source text NOT NULL
);
CREATE INDEX IF NOT EXISTS river_levels_at_idx ON river_levels (at DESC);
CREATE TABLE IF NOT EXISTS flood_zones (
  id text PRIMARY KEY, name text NOT NULL, river text, lat double precision, lng double precision, note text, node text, sources text[], source text NOT NULL DEFAULT 'documented'
);
CREATE TABLE IF NOT EXISTS landslides (
  id text PRIMARY KEY, name text NOT NULL, road text, lat double precision, lng double precision, note text, sources text[], source text NOT NULL DEFAULT 'documented'
);
CREATE TABLE IF NOT EXISTS zoning (
  class text PRIMARY KEY, share double precision NOT NULL, tone text, label text, source text NOT NULL
);
CREATE TABLE IF NOT EXISTS flood_state (
  id bigserial PRIMARY KEY, stage double precision NOT NULL, at timestamptz NOT NULL DEFAULT now(), source text NOT NULL
);
