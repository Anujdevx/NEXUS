CREATE TABLE IF NOT EXISTS incidents (
  id text PRIMARY KEY, kind text NOT NULL, title text NOT NULL, place text, sev text, lat double precision, lng double precision,
  node text, status text NOT NULL DEFAULT 'Open', unit_id text, sim boolean NOT NULL DEFAULT false, scenario text,
  detail text, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS incident_log (
  id bigserial PRIMARY KEY, incident_id text NOT NULL REFERENCES incidents(id) ON DELETE CASCADE, at timestamptz NOT NULL DEFAULT now(), text text NOT NULL
);
CREATE INDEX IF NOT EXISTS incident_log_incident_idx ON incident_log (incident_id, id);
CREATE TABLE IF NOT EXISTS sos (
  id text PRIMARY KEY, place text, hazard text, people int, injured boolean, lat double precision, lng double precision,
  via text, raised_at timestamptz NOT NULL DEFAULT now(), status text NOT NULL DEFAULT 'received', client text
);
CREATE TABLE IF NOT EXISTS reports (
  id uuid PRIMARY KEY, user_id text, type text NOT NULL, lat double precision, lng double precision, description text,
  status text NOT NULL DEFAULT 'new', media_id text, created_at timestamptz NOT NULL DEFAULT now()
);
