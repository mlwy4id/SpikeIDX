CREATE TABLE IF NOT EXISTS ohlcv (
  code   TEXT NOT NULL REFERENCES stocks_master(code),
  date   DATE NOT NULL,
  open   NUMERIC NOT NULL,
  high   NUMERIC NOT NULL,
  low    NUMERIC NOT NULL,
  close  NUMERIC NOT NULL,
  volume BIGINT NOT NULL,
  raw    JSONB,
  PRIMARY KEY (code, date)
);

CREATE INDEX IF NOT EXISTS idx_ohlcv_code_date ON ohlcv (code, date DESC);
