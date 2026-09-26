CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS stocks_master (
  code         TEXT PRIMARY KEY,
  yahoo_symbol TEXT NOT NULL,
  name         TEXT NOT NULL,
  sector       TEXT,
  updated_at   TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS watchlist (
  user_id    TEXT NOT NULL DEFAULT 'default',
  code       TEXT NOT NULL REFERENCES stocks_master(code),
  created_at TIMESTAMPTZ DEFAULT now(),
  PRIMARY KEY (user_id, code)
);

CREATE INDEX IF NOT EXISTS idx_stocks_name_trgm ON stocks_master USING gin (name gin_trgm_ops);
