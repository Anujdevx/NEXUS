CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY,
  email text UNIQUE NOT NULL,
  password_hash text NOT NULL,
  role text NOT NULL CHECK (role IN ('Controller','Responder','Citizen')),
  created_at timestamptz NOT NULL DEFAULT now()
);
