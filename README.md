# SpikeIDX — deteksi volume spike saham Indonesia (EOD)

Go clean-lite + Postgres (cloud) + Yahoo `.JK` primary.

## Layout

- `cmd/api` — REST via Gin: search, watchlist, signals
- `internal/http` — Gin handlers tipis di atas usecase (validasi + map error: 400/404/409/502)
- `cmd/worker` — v2 (ditunda): cron EOD fetch → upsert → detect spike + ADL → Telegram
- `internal/domain` — entities + interfaces (`StockRepository` wajib sebelum tulis watchlist/ohlcv)
- `internal/usecase` — spike rule (2x SMA20 + z>2 + filter |change|>2%), Chaikin A/D Line + label rasio akumulasi/distribusi (window 20, ±0.10)
- `internal/infra/yahoo` — Yahoo chart + search (browser UA, 1 req/s)
- `internal/infra/idx` — stub fallback IDX (v2: broker summary)
- `internal/infra/postgres` — repo pgx + auto-migrate saat startup (strict, tanpa fallback)
- `internal/infra/postgres/migrations` — DDL kanonis (manual: `psql -f internal/infra/postgres/migrations/001_master.sql`)

## Run lokal

```bash
go test ./...
golangci-lint run ./...   # lint bersih (config: .golangci.yml)
cp .env.example .env   # isi DATABASE_URL cloud
go run ./cmd/api       # :8080, gagal keras tanpa DB (WireStrict)
```

Smoke: `GET /health` → `{"status":"ok"}`; `POST /api/v1/watchlist {"code":"X"}` tanpa search → 404.

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
Backend strict-Postgres via `infra.WireStrict`: gagal keras tanpa `DATABASE_URL` yang reachable.
Worker v2 cron: `30 16 * * 1-5 TZ=Asia/Jakarta ./worker`.

## API

- `GET /api/v1/search?q=bank`
- `GET /api/v1/watchlist` / `POST /api/v1/watchlist {"code":"BBCA"}` / `DELETE /api/v1/watchlist/BBCA`
- `GET /api/v1/signals?date=2026-09-18`
- `GET /api/v1/ohlcv/BBCA?limit=60` (histori oldest-first, default 60, cap 500)

Docs: Swagger UI di `GET /swagger/index.html` (alias `/swagger`, `/docs`), spec mentah di `GET /openapi.yaml` (sumber: `internal/http/openapi.yaml`).

## Next

Rencana aktif: `docs/IMPLEMENTATION_PLAN.md` (§8: label akumulasi/distribusi rasio-CMF).

1. E2E cloud via contract test (`DATABASE_URL` user) + smoke Yahoo nyata (search → watchlist → backfill → `GET /api/v1/ohlcv/BBCA` + `GET /api/v1/signals`).
2. Seed `stocks_master` IDX (idempoten) + uji fetch Yahoo 20–100 simbol (ukur 429/latency).
3. Retry/backoff Yahoo (429/5xx, stdlib-only, di `internal/infra/yahoo` saja).
4. `cmd/worker` v2 + IDX `GetStockSummary` fallback + `GetBrokerSummary` v2; kalender libur BEI penuh + `robfig/cron` bila worker jadi long-running.

## Worker-minimal

Single-run EOD: `go run ./cmd/worker` (butuh DB). Skip eksplisit bila weekend (kalender `nil`, weekend-only) atau watchlist kosong — exit 0, tanpa cron/`Send`. Histori via `GET /api/v1/ohlcv/:code` (oldest-first, default 60, cap 500).
DB-down/list-gagal → exit non-zero (`wire:`/`worker: list watchlist:`) untuk cron-alert. E2E/migrasi: jalankan api lalu worker secara sekuensial, jangan konkuren agar auto-migrate tidak balapan.
