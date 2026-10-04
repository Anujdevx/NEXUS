CREATE TABLE IF NOT EXISTS event_log (
  id uuid PRIMARY KEY,
  routing_key text NOT NULL,
  envelope jsonb NOT NULL,
  received_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS event_log_received_at_idx ON event_log (received_at DESC);
