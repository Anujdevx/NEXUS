CREATE TABLE IF NOT EXISTS hospitals (
  id text PRIMARY KEY, name text NOT NULL, own text, lvl text, lat double precision, lng double precision,
  loc text, phone text, beds int, node text,
  cap int NOT NULL, free int NOT NULL, inbound int NOT NULL DEFAULT 0, divert boolean NOT NULL DEFAULT false,
  source text NOT NULL DEFAULT 'nic'
);
CREATE TABLE IF NOT EXISTS units (
  id text PRIMARY KEY, type text NOT NULL, node text, status text NOT NULL DEFAULT 'Available', task text NOT NULL DEFAULT '',
  lat double precision, lng double precision, step text
);
CREATE TABLE IF NOT EXISTS shelters (
  id text PRIMARY KEY, name text NOT NULL, node text, lat double precision, lng double precision,
  cap int NOT NULL, occ int NOT NULL, open boolean NOT NULL DEFAULT true
);
CREATE TABLE IF NOT EXISTS pharmacies (
  id text PRIMARY KEY, name text NOT NULL, lat double precision, lng double precision, loc text, hrs text
);
CREATE TABLE IF NOT EXISTS stock (
  pharmacy_id text NOT NULL REFERENCES pharmacies(id), med_id text NOT NULL, qty int NOT NULL,
  PRIMARY KEY (pharmacy_id, med_id)
);
CREATE TABLE IF NOT EXISTS assignments (
  id text PRIMARY KEY, incident_id text, unit_id text, hospital_id text, route jsonb,
  approved_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS assignments_incident_idx ON assignments (incident_id);
