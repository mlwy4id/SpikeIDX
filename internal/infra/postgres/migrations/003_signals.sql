CREATE TABLE IF NOT EXISTS signals (
  code        TEXT NOT NULL,
  date        DATE NOT NULL,
  volume      BIGINT NOT NULL,
  avg20       NUMERIC NOT NULL,
  multiple    NUMERIC NOT NULL,
  zscore      NUMERIC NOT NULL,
  close       NUMERIC NOT NULL,
  pct_change  NUMERIC NOT NULL,
  adl         NUMERIC NOT NULL,
  adl_slope5  NUMERIC NOT NULL,
  is_filtered BOOLEAN NOT NULL DEFAULT FALSE,
  created_at  TIMESTAMPTZ DEFAULT now(),
  PRIMARY KEY (code, date)
);

CREATE INDEX IF NOT EXISTS idx_signals_date ON signals (date DESC);
