CREATE TABLE IF NOT EXISTS alerts (
  id text PRIMARY KEY,
  area text NOT NULL,
  severity text NOT NULL,
  text_en text NOT NULL,
  text_hi text NOT NULL DEFAULT '',
  issued_at timestamptz NOT NULL DEFAULT now(),
  issued_by text NOT NULL,
  source text NOT NULL DEFAULT 'manual',
  cap jsonb
);
CREATE INDEX IF NOT EXISTS alerts_issued_idx ON alerts (issued_at DESC);
