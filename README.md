# SpikeIDX — deteksi volume spike saham Indonesia (EOD)

Go clean-lite + Postgres (cloud) + Yahoo `.JK` primary.

## Layout

- `cmd/api` — REST via Gin: search, watchlist, signals
- `cmd/worker` — cron EOD: fetch → upsert → detect spike + ADL → Telegram
- `internal/domain` — entities + interfaces (`StockRepository` wajib sebelum tulis watchlist/ohlcv)
- `internal/usecase` — spike rule (2x SMA20 + z>2 + filter |change|>2%), Chaikin A/D Line
- `internal/infra/yahoo` — Yahoo chart + search (browser UA, 1 req/s)
- `internal/infra/idx` — stub fallback IDX (v2: broker summary)
- `internal/infra/memory` — in-memory repos (dev tanpa DB)
- `internal/infra/postgres` — repo pgx + auto-migrate saat startup
- `internal/infra/postgres/migrations` — DDL kanonis (`db/migrations` berisi symlink untuk `psql -f` manual)

## Run lokal (tanpa DB)

```bash
go test ./...
go run ./cmd/api            # :8080, backend=memory, GET /health
go run ./cmd/worker         # jalan sekali (dev default BBCA/TLKM/BBRI bila watchlist kosong)
```

Alur wajib search dulu: `GET /search?q=BBCA` (auto-cache master) → `POST /watchlist {"code":"BBCA"}` → backfill jalan di background.

## Postgres cloud (Supabase / Neon)

1. Buat project Postgres cloud (region terdekat, mis. Singapore).
2. Ambil connection string **Session mode port 5432** (bukan Transaction pooler 6543, agar `CREATE EXTENSION pg_trgm` + migrasi jalan).
3. Isi `.env` dari `.env.example` (`DATABASE_URL=...?sslmode=require`).
4. Jalankan — migrasi (`001-003`) diterapkan otomatis oleh app saat startup:
```bash
cp .env.example .env   # isi DATABASE_URL
go run ./cmd/api       # log: backend=postgres
```
Produksi/docker: set `STRICT_DB=1` agar app gagal keras (bukan fallback memory) bila DB tidak reachable. Cron worker: `30 16 * * 1-5 TZ=Asia/Jakarta ./worker`.

## API

- `GET /api/v1/search?q=bank`
- `GET /api/v1/watchlist` / `POST /api/v1/watchlist {"code":"BBCA"}` / `DELETE /api/v1/watchlist/BBCA`
- `GET /api/v1/signals?date=2026-09-18`

## Next

1. Seed `stocks_master` IDX + uji fetch Yahoo 20–100 simbol.
2. Tambah IDX `GetStockSummary` fallback + `GetBrokerSummary` v2.
3. Kalender libur BEI + `ByDate(time.Time)` + validasi kode (temuan review domain #2–4).
4. Tambah `robfig/cron` in-process bila worker jadi long-running.
