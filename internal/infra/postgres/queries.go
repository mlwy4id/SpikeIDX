package postgres

const UpsertStock = `
INSERT INTO stocks_master (code, yahoo_symbol, name, sector)
VALUES ($1,$2,$3,$4)
ON CONFLICT (code) DO UPDATE SET
  yahoo_symbol=EXCLUDED.yahoo_symbol, name=EXCLUDED.name,
  sector=EXCLUDED.sector, updated_at=now()`

const UpsertOHLCV = `
INSERT INTO ohlcv (code, date, open, high, low, close, volume)
VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (code, date) DO UPDATE SET
  open=EXCLUDED.open, high=EXCLUDED.high, low=EXCLUDED.low,
  close=EXCLUDED.close, volume=EXCLUDED.volume`

const SpikeCandidates = `
WITH base AS (
  SELECT code, date, close, volume,
    AVG(volume) OVER w AS avg20,
    STDDEV_POP(volume) OVER w AS std20,
    LAG(close) OVER (PARTITION BY code ORDER BY date) AS prev_close
  FROM ohlcv
  WINDOW w AS (PARTITION BY code ORDER BY date ROWS 19 PRECEDING)
)
SELECT code, date, volume, avg20,
  volume / NULLIF(avg20,0) AS multiple,
  (volume - avg20) / NULLIF(std20,0) AS zscore,
  (close - prev_close) / NULLIF(prev_close,0) * 100 AS pct_change
FROM base WHERE date = $1`
